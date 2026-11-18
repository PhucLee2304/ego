package service

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	learningSocket "ego/services/learning/internal/socket"
)

// ErrParticipantRemoved: học sinh đã bị mời ra khỏi buổi học, không được vào lại hay thao tác
// cho đến khi buổi học kết thúc.
var ErrParticipantRemoved = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] You were removed from this session by the teacher", nil)

// participantPermissions tính quyền publish của học sinh từ state; state nil
// nghĩa là chưa bị giới hạn gì.
func participantPermissions(state *model.RoomParticipantState, screenShare bool) media.PublishPermissions {
	if state == nil {
		return media.PublishPermissions{
			Camera:      true,
			Microphone:  true,
			ScreenShare: screenShare}
	}
	return media.PublishPermissions{
		Camera:      state.CameraAllowed,
		Microphone:  state.MicrophoneAllowed,
		ScreenShare: screenShare}
}

// ignoreNotConnected coi "người không còn trong phòng" là xong việc (không có gì để ngắt hay
// tắt nữa); lỗi LiveKit khác vẫn trả về qua mediaError.
func ignoreNotConnected(err error) error {
	if err == nil || errors.Is(err, media.ErrParticipantNotFound) {
		return nil
	}
	return mediaError(err)
}

// moderationTarget kiểm tra người gọi là giáo viên của room đang LIVE và
// [userID] là học sinh của lớp chưa bị xóa khỏi phòng.
func (s *service) moderationTarget(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*model.Room, *model.RoomParticipantState, error) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, nil, err
	}
	if !isHost {
		return nil, nil, ErrClassroomForbidden
	}
	if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
		return nil, nil, err
	}
	if s.mediaClient == nil {
		return nil, nil, ErrMediaUnavailable
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, nil, err
	}
	return room, state, nil
}

// MuteParticipant cho giáo viên tắt mic của một học sinh; học sinh không tự bật lại được
// cho đến khi giáo viên cho phép.
func (s *service) MuteParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceMicrophone, false)
	return response, httpx.FromError(err, errClassroomServiceFailed)
}

// AllowParticipantMicrophone cho học sinh được bật mic lại; mic không tự bật, học sinh tự bật.
func (s *service) AllowParticipantMicrophone(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceMicrophone, true)
	return response, httpx.FromError(err, errClassroomServiceFailed)
}

// DisableParticipantCamera cho giáo viên tắt camera của một học sinh và khóa việc bật lại.
func (s *service) DisableParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceCamera, false)
	return response, httpx.FromError(err, errClassroomServiceFailed)
}

// AllowParticipantCamera cho học sinh được bật camera lại; camera không tự bật.
func (s *service) AllowParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceCamera, true)
	return response, httpx.FromError(err, errClassroomServiceFailed)
}

// setDevicePermission là phần chung của bốn API bật/tắt mic, camera: kiểm người gọi là giáo
// viên, học sinh chưa bị mời ra, rồi áp quyền.
func (s *service) setDevicePermission(ctx context.Context, teacherID string, classroomID, roomID uint, userID string, source media.Source, allowed bool) (*dto.RoomParticipantStateResponse, error) {
	room, state, err := s.moderationTarget(ctx, teacherID, classroomID, roomID, userID)
	if err != nil {
		return nil, err
	}
	if state.RemovedAt != nil {
		return nil, ErrParticipantRemoved
	}
	if _, err := s.applyDevicePermission(ctx, room, teacherID, state, source, allowed); err != nil {
		return nil, err
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

// syncParticipantPermissions gửi LiveKit bộ quyền publish (mic, camera, chia sẻ màn hình)
// tính từ state mới nhất trong DB. Mọi thay đổi quyền lưu DB trước rồi gọi hàm này sau
// commit (side effect ngoài chạy sau commit, không giữ khóa dòng khi gọi LiveKit). Các lượt
// đồng bộ của cùng học sinh chạy tuần tự và đọc state bên trong khóa, nên lượt chạy sau
// cùng luôn thấy thay đổi sau cùng: LiveKit hội tụ về DB dù giáo viên tắt mic đúng lúc
// duyệt/dừng chia sẻ màn hình. Khóa trong tiến trình vì learning chạy một instance.
//
// Lỗi DB trả nguyên; lỗi LiveKit đã qua mediaError. Học sinh chưa/không còn kết nối trả
// ErrParticipantNotConnected khi [requireConnected], không thì coi như xong (quyền đã lưu,
// áp dụng vào token/lần đồng bộ lúc vào phòng).
func (s *service) syncParticipantPermissions(ctx context.Context, room *model.Room, userID string, requireConnected bool) error {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%d:%s", room.ID, userID)
	mu := &s.permissionLocks[h.Sum32()%uint32(len(s.permissionLocks))]
	mu.Lock()
	defer mu.Unlock()
	state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return err
	}
	// Đã bị mời ra thì đã bị đá khỏi LiveKit; không còn phiên để cấp quyền.
	if state != nil && state.RemovedAt != nil {
		return nil
	}
	permissions := participantPermissions(state, state != nil && state.ScreenShareApprovedAt != nil)
	err = s.mediaClient.SetPublishPermissions(ctx, media.RoomName(room.ID), userID, permissions)
	if err == nil || (!requireConnected && errors.Is(err, media.ErrParticipantNotFound)) {
		return nil
	}
	return mediaError(err)
}

// applyDevicePermission lưu quyền và audit trong một tx (khóa dòng để đọc-rồi-ghi không
// lệch), rồi sau commit mới tắt track và đồng bộ quyền LiveKit. Giáo viên chỉ cấp lại
// quyền; học sinh vẫn phải tự bật thiết bị của mình. Quyền không đổi vẫn đồng bộ lại để
// lần gọi lặp sửa được LiveKit nếu lần trước lỗi sau commit.
func (s *service) applyDevicePermission(ctx context.Context, room *model.Room, teacherID string, state *model.RoomParticipantState, source media.Source, allowed bool) (bool, error) {
	column, action := repository.ParticipantColMicrophone, model.RoomModerationAuditActionMute
	if source == media.SourceCamera {
		column, action = repository.ParticipantColCamera, model.RoomModerationAuditActionDisableCamera
	}
	changed := false
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := repo.LockRoomParticipantState(ctx, room.ID, state.UserID)
		if err != nil {
			return err
		}
		if locked == nil || locked.RemovedAt != nil {
			return ErrParticipantRemoved
		}
		next := *locked
		current := &next.MicrophoneAllowed
		if source == media.SourceCamera {
			current = &next.CameraAllowed
		}
		if *current == allowed {
			*state = next
			return nil
		}
		*current = allowed
		if err := repo.UpdateRoomParticipantState(ctx, &next, column); err != nil {
			return err
		}
		if !allowed {
			if err := repo.CreateRoomModerationAudit(ctx, &model.RoomModerationAudit{RoomID: room.ID, ActorUserID: teacherID, TargetUserID: state.UserID, Action: action}); err != nil {
				return err
			}
		}
		*state, changed = next, true
		return nil
	}); err != nil {
		return false, err
	}
	if changed {
		logger.Log.Info().Uint("room_id", room.ID).Str("actor_id", teacherID).Str("user_id", state.UserID).
			Bool("microphone_allowed", state.MicrophoneAllowed).Bool("camera_allowed", state.CameraAllowed).Msg("[ROOM] Participant permissions updated")
		eventType := learningSocket.EventParticipantPermissionsUpdated
		if allowed && source == media.SourceMicrophone {
			eventType = learningSocket.EventParticipantMicrophoneAllowed
		}
		s.publishRoomActivity(room, eventType, state.UserID)
	}
	// Học sinh có thể đang không kết nối; quyền vẫn được lưu và áp dụng vào
	// token ở lần vào phòng kế tiếp.
	if !allowed {
		if err := ignoreNotConnected(s.mediaClient.MuteTracks(ctx, media.RoomName(room.ID), state.UserID, source)); err != nil {
			return changed, err
		}
	}
	return changed, s.syncParticipantPermissions(ctx, room, state.UserID, false)
}

// MuteAllParticipants tắt mic mọi học sinh đã vào phòng và chưa bị tắt.
func (s *service) MuteAllParticipants(ctx context.Context, teacherID string, classroomID, roomID uint) (*dto.MutedParticipantsResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	states, err := s.repo.GetRoomParticipantStates(ctx, room.ID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Lỗi LiveKit sau khi đã lưu không dừng cả lượt (các bạn sau vẫn được tắt); chỉ lỗi DB
	// mới trả lỗi. Bạn đã tắt mic trong DB được bỏ qua: LiveKit lỗi ở lần trước được sửa khi
	// học sinh vào lại phòng (đồng bộ lúc join) hoặc khi tắt mic riêng bạn đó.
	muted := []string{}
	for _, state := range states {
		if state.UserID == teacherID || state.RemovedAt != nil || !state.MicrophoneAllowed {
			continue
		}
		changed, err := s.applyDevicePermission(ctx, room, teacherID, state, media.SourceMicrophone, false)
		if changed {
			muted = append(muted, state.UserID)
		}
		// Vừa bị mời ra khỏi phòng trong lúc tắt mic cả lớp: bỏ qua.
		if err == nil || errors.Is(err, ErrParticipantRemoved) {
			continue
		}
		if !errors.Is(err, ErrMediaUnavailable) && !errors.Is(err, ErrParticipantNotConnected) {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		logger.Log.Warn().Err(err).Uint("room_id", room.ID).Str("user_id", state.UserID).Msg("[ERROR] Failed to mute participant in media server")
	}
	return &dto.MutedParticipantsResponse{UserIDs: muted}, nil
}

// RemoveParticipant đưa học sinh ra khỏi phòng (không xóa khỏi lớp) và khóa
// việc vào lại cho đến khi room kết thúc.
func (s *service) RemoveParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string, reason *string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	room, state, err := s.moderationTarget(ctx, teacherID, classroomID, roomID, userID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if reason != nil {
		trimmed := strings.TrimSpace(*reason)
		reason = &trimmed
		if trimmed == "" {
			reason = nil
		}
	}
	// Khóa dòng để hai lần mời ra cùng lúc chỉ một lần ghi audit và push; LiveKit đá ra sau
	// commit, và cả khi đã bị mời ra từ trước (lần gọi lặp sửa được lần đá lỗi trước đó).
	removed := false
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := repo.LockRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if locked != nil {
			*state = *locked
		}
		if state.RemovedAt != nil {
			return nil
		}
		now := time.Now()
		state.RemovedAt, state.RemovedReason = &now, reason
		state.HandRaisedAt, state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = nil, nil, nil
		if err := repo.UpdateRoomParticipantState(ctx, state, repository.ParticipantColRemovedAt, repository.ParticipantColRemovedReason,
			repository.ParticipantColHandRaisedAt, repository.ParticipantColShareRequestedAt, repository.ParticipantColShareApprovedAt); err != nil {
			return err
		}
		removed = true
		return repo.CreateRoomModerationAudit(ctx, &model.RoomModerationAudit{RoomID: room.ID, ActorUserID: teacherID, TargetUserID: userID, Action: model.RoomModerationAuditActionRemove, Reason: reason})
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if removed {
		logger.Log.Info().Uint("room_id", room.ID).Str("actor_id", teacherID).Str("user_id", userID).Msg("[ROOM] Participant removed from the session")
		s.publishRoomActivity(room, learningSocket.EventParticipantRemoved, userID)
	}
	if err := ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), userID)); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}
