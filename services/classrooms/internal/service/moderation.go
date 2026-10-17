package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"ego/platform/logger"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/media"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	classroomsSocket "ego/services/classrooms/internal/socket"
)

var ErrParticipantRemoved = errors.New("[FORBIDDEN] You were removed from this session by the teacher")

// participantPermissions tính quyền publish của học sinh từ state; state nil
// nghĩa là chưa bị giới hạn gì.
func participantPermissions(state *model.RoomParticipantState, screenShare bool) media.PublishPermissions {
	if state == nil {
		return media.PublishPermissions{Camera: true, Microphone: true, ScreenShare: screenShare}
	}
	return media.PublishPermissions{Camera: state.CameraAllowed, Microphone: state.MicrophoneAllowed, ScreenShare: screenShare}
}

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

func (s *service) MuteParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	return s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceMicrophone, false)
}

func (s *service) AllowParticipantMicrophone(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	return s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceMicrophone, true)
}

func (s *service) DisableParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	return s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceCamera, false)
}

func (s *service) AllowParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	return s.setDevicePermission(ctx, teacherID, classroomID, roomID, userID, media.SourceCamera, true)
}

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

// applyDevicePermission đổi quyền trên LiveKit trước rồi mới lưu state và
// audit, nên audit chỉ ghi khi thao tác quản trị đã thành công. Giáo viên chỉ
// cấp lại quyền; học sinh vẫn phải tự bật thiết bị của mình.
func (s *service) applyDevicePermission(ctx context.Context, room *model.Room, teacherID string, state *model.RoomParticipantState, source media.Source, allowed bool) (bool, error) {
	current := state.MicrophoneAllowed
	if source == media.SourceCamera {
		current = state.CameraAllowed
	}
	if current == allowed {
		return false, nil
	}
	next := *state
	if source == media.SourceCamera {
		next.CameraAllowed = allowed
	} else {
		next.MicrophoneAllowed = allowed
	}
	roomName := media.RoomName(room.ID)
	// Học sinh có thể đang không kết nối; quyền vẫn được lưu và áp dụng vào
	// token ở lần vào phòng kế tiếp.
	if !allowed {
		if err := ignoreNotConnected(s.mediaClient.MuteTracks(ctx, roomName, state.UserID, source)); err != nil {
			return false, err
		}
	}
	permissions := participantPermissions(&next, next.ScreenShareApprovedAt != nil)
	if err := ignoreNotConnected(s.mediaClient.SetPublishPermissions(ctx, roomName, state.UserID, permissions)); err != nil {
		return false, err
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.UpdateRoomParticipantState(ctx, &next); err != nil {
			return err
		}
		if allowed {
			return nil
		}
		action := model.RoomModerationAuditActionMute
		if source == media.SourceCamera {
			action = model.RoomModerationAuditActionDisableCamera
		}
		return repo.CreateRoomModerationAudit(ctx, &model.RoomModerationAudit{RoomID: room.ID, ActorUserID: teacherID, TargetUserID: state.UserID, Action: action})
	}); err != nil {
		return false, err
	}
	*state = next
	logger.Log.Info().Uint("room_id", room.ID).Str("actor_id", teacherID).Str("user_id", state.UserID).
		Bool("microphone_allowed", state.MicrophoneAllowed).Bool("camera_allowed", state.CameraAllowed).Msg("[ROOM] Participant permissions updated")
	eventType := classroomsSocket.EventParticipantPermissionsUpdated
	if allowed && source == media.SourceMicrophone {
		eventType = classroomsSocket.EventParticipantMicrophoneAllowed
	}
	s.publishRoomActivity(room, eventType, state.UserID)
	return true, nil
}

// MuteAllParticipants tắt mic mọi học sinh đã vào phòng và chưa bị tắt.
func (s *service) MuteAllParticipants(ctx context.Context, teacherID string, classroomID, roomID uint) (*dto.MutedParticipantsResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	states, err := s.repo.GetRoomParticipantStates(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	muted := []string{}
	for _, state := range states {
		if state.UserID == teacherID || state.RemovedAt != nil || !state.MicrophoneAllowed {
			continue
		}
		changed, err := s.applyDevicePermission(ctx, room, teacherID, state, media.SourceMicrophone, false)
		if err != nil {
			return nil, err
		}
		if changed {
			muted = append(muted, state.UserID)
		}
	}
	return &dto.MutedParticipantsResponse{UserIDs: muted}, nil
}

// RemoveParticipant đưa học sinh ra khỏi phòng (không xóa khỏi lớp) và khóa
// việc vào lại cho đến khi room kết thúc.
func (s *service) RemoveParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string, reason *string) (*dto.RoomParticipantStateResponse, error) {
	room, state, err := s.moderationTarget(ctx, teacherID, classroomID, roomID, userID)
	if err != nil {
		return nil, err
	}
	if state.RemovedAt != nil {
		return dto.ToRoomParticipantStateResponse(state), nil
	}
	if err := ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), userID)); err != nil {
		return nil, err
	}
	if reason != nil {
		trimmed := strings.TrimSpace(*reason)
		reason = &trimmed
		if trimmed == "" {
			reason = nil
		}
	}
	now := time.Now()
	next := *state
	next.RemovedAt = &now
	next.HandRaisedAt, next.ScreenShareRequestedAt, next.ScreenShareApprovedAt = nil, nil, nil
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.UpdateRoomParticipantState(ctx, &next); err != nil {
			return err
		}
		return repo.CreateRoomModerationAudit(ctx, &model.RoomModerationAudit{RoomID: room.ID, ActorUserID: teacherID, TargetUserID: userID, Action: model.RoomModerationAuditActionRemove, Reason: reason})
	}); err != nil {
		return nil, err
	}
	logger.Log.Info().Uint("room_id", room.ID).Str("actor_id", teacherID).Str("user_id", userID).Msg("[ROOM] Participant removed from the session")
	s.publishRoomActivity(room, classroomsSocket.EventParticipantRemoved, userID)
	return dto.ToRoomParticipantStateResponse(&next), nil
}
