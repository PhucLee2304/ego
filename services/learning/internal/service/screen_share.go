package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"

	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	learningSocket "ego/services/learning/internal/socket"
)

// Lỗi nghiệp vụ của luồng chia sẻ màn hình, trả thẳng cho HTTP.
var (
	ErrScreenShareTeacherDirect = httpx.NewError(http.StatusBadRequest, "[ERROR] The teacher can share the screen directly without a request", nil)
	ErrScreenShareNotRequested  = httpx.NewError(http.StatusConflict, "[CONFLICT] This participant has no screen share request", nil)
	ErrScreenShareNotApproved   = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Screen sharing has not been approved by the teacher", nil)
	ErrRoomParticipantInvalid   = httpx.NewError(http.StatusBadRequest, "[ERROR] The user is not a student of this classroom", nil)
	ErrParticipantNotConnected  = httpx.NewError(http.StatusConflict, "[CONFLICT] The participant is not connected to the room", nil)
)

// clearScreenShareState xóa yêu cầu và phê duyệt chia sẻ màn hình của userID, để lần chia sẻ
// sau phải xin lại. Trả true nếu thật sự có gì để xóa, để caller chỉ báo sự kiện khi state đổi.
func (s *service) clearScreenShareState(ctx context.Context, roomID uint, userID string) (bool, error) {
	state, err := s.repo.FindRoomParticipantState(ctx, roomID, userID)
	if err != nil || state == nil {
		return false, err
	}
	if state.ScreenShareRequestedAt == nil && state.ScreenShareApprovedAt == nil {
		return false, nil
	}
	state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = nil, nil
	return true, s.repo.UpdateRoomParticipantState(ctx, state, repository.ParticipantColShareRequestedAt, repository.ParticipantColShareApprovedAt)
}

// publishRoomActivity chỉ gửi cho giáo viên và người liên quan vì học sinh chỉ
// được xem state của chính mình.
func (s *service) publishRoomActivity(room *model.Room, eventType, userID string) {
	if s.roomPublisher == nil {
		return
	}
	event := learningSocket.NewRoomEvent(eventType, room.ID, learningSocket.ParticipantActivityPayload{UserID: userID})
	teacherID := room.Schedule.Classroom.TeacherID
	s.roomPublisher(teacherID, event)
	if userID != teacherID {
		s.roomPublisher(userID, event)
	}
}

// RequestScreenShare cho học sinh xin chia sẻ màn hình; giáo viên được chia sẻ thẳng nên
// không cần xin. Gửi lại khi đang chờ hoặc đã được duyệt thì trả state hiện tại, không tạo yêu cầu mới.
func (s *service) RequestScreenShare(ctx context.Context, userID string, classroomID, roomID uint) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if isHost {
		return nil, ErrScreenShareTeacherDirect
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if state.RemovedAt != nil {
		return nil, ErrParticipantRemoved
	}
	// Gửi lại khi đang chờ hoặc đã được duyệt không tạo yêu cầu mới. UPDATE có điều kiện:
	// bấm hai lần cùng lúc chỉ một request tạo yêu cầu và push.
	if state.ScreenShareRequestedAt == nil {
		created, err := s.repo.RequestRoomScreenShare(ctx, room.ID, userID, time.Now())
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if created {
			s.publishRoomActivity(room, learningSocket.EventScreenShareRequested, userID)
		}
		if state, err = s.repo.FindRoomParticipantState(ctx, room.ID, userID); err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if state.RemovedAt != nil {
			return nil, ErrParticipantRemoved
		}
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

// ApproveScreenShare cho giáo viên duyệt yêu cầu chia sẻ màn hình của học sinh. Phê duyệt
// lưu DB trước, rồi cấp quyền trên LiveKit; chỉ báo đã duyệt khi LiveKit cấp xong, để học sinh
// không thấy được duyệt mà vẫn không publish được.
func (s *service) ApproveScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Khóa dòng để kiểm lại yêu cầu trên state mới nhất (học sinh vừa rời phòng/bị mời ra
	// thì không duyệt nữa). Quyền LiveKit cấp sau commit: học sinh chỉ publish được screen
	// share khi LiveKit đã cho phép, không dựa vào việc client tự kiểm tra; APPROVED chỉ
	// được push khi LiveKit đã cấp xong.
	var state *model.RoomParticipantState
	var approvedAt *time.Time // khác nil khi lượt này vừa duyệt
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := repo.LockRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if locked == nil || locked.ScreenShareRequestedAt == nil || locked.RemovedAt != nil {
			return ErrScreenShareNotRequested
		}
		state = locked
		if state.ScreenShareApprovedAt != nil {
			return nil
		}
		if s.mediaClient == nil {
			return ErrMediaUnavailable
		}
		// Postgres lưu tới micro giây: cắt trước để RevertRoomScreenShareApproval so khớp được.
		now := time.Now().Truncate(time.Microsecond)
		state.ScreenShareApprovedAt, approvedAt = &now, &now
		return repo.UpdateRoomParticipantState(ctx, state, repository.ParticipantColShareApprovedAt)
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Đã duyệt từ trước mà không có LiveKit: trả state như cũ. Có LiveKit thì đồng bộ (cả lần
	// duyệt lặp, để sửa được lần cấp quyền lỗi trước đó) rồi mới báo đã duyệt.
	if s.mediaClient == nil {
		return dto.ToRoomParticipantStateResponse(state), nil
	}
	if err := s.syncParticipantPermissions(ctx, room, userID, true); err != nil {
		// LiveKit không cấp được (học sinh vừa rớt kết nối…): hủy đúng lượt duyệt này để yêu
		// cầu về chờ duyệt (StartScreenShare chỉ xem DB), rồi đồng bộ lại vì lượt đồng bộ khác
		// (webhook join…) có thể đã cấp quyền theo lượt duyệt vừa hủy.
		if approvedAt != nil {
			if revertErr := s.repo.RevertRoomScreenShareApproval(ctx, room.ID, userID, *approvedAt); revertErr != nil {
				err = errors.Join(err, revertErr)
			} else if syncErr := s.syncParticipantPermissions(ctx, room, userID, false); syncErr != nil {
				err = errors.Join(err, syncErr)
			}
		}
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Chỉ báo khi lượt này vừa duyệt: duyệt lặp không push lại.
	if approvedAt != nil {
		s.publishRoomActivity(room, learningSocket.EventScreenShareApproved, userID)
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

// RejectScreenShare cho giáo viên từ chối hoặc thu hồi yêu cầu chia sẻ màn hình; nếu đã
// duyệt thì dừng luôn phần đang chia sẻ.
func (s *service) RejectScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Khóa dòng như ApproveScreenShare: duyệt và từ chối cùng lúc thì từ chối thấy đúng việc
	// đã duyệt hay chưa, không để LiveKit còn cấp quyền khi DB đã xóa yêu cầu.
	var state *model.RoomParticipantState
	wasApproved := false
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		locked, err := repo.LockRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if locked == nil || locked.ScreenShareRequestedAt == nil {
			return ErrScreenShareNotRequested
		}
		state, wasApproved = locked, locked.ScreenShareApprovedAt != nil
		if wasApproved && s.mediaClient == nil {
			return ErrMediaUnavailable
		}
		state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = nil, nil
		return repo.UpdateRoomParticipantState(ctx, state, repository.ParticipantColShareRequestedAt, repository.ParticipantColShareApprovedAt)
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Từ chối sau khi đã duyệt cũng thu hồi quyền và dừng phần đang chia sẻ.
	if wasApproved {
		if err := s.stopScreenShareMedia(ctx, room, userID); err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
	}
	s.publishRoomActivity(room, learningSocket.EventScreenShareRejected, userID)
	return dto.ToRoomParticipantStateResponse(state), nil
}

// StartScreenShare được gọi ngay trước khi client publish screen share. Mỗi
// phòng chỉ có một người chia sẻ nên backend dừng lượt đang chạy trước.
func (s *service) StartScreenShare(ctx context.Context, userID string, classroomID, roomID uint) *httpx.HTTPError {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	if !isHost {
		state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return httpx.FromError(err, errClassroomServiceFailed)
		}
		if state == nil || state.ScreenShareApprovedAt == nil {
			return ErrScreenShareNotApproved
		}
	}
	return httpx.FromError(s.stopOtherScreenShares(ctx, room, userID), errClassroomServiceFailed)
}

// StopScreenShare dừng chia sẻ màn hình: người đang chia sẻ tự dừng, hoặc giáo viên dừng
// của học sinh. Học sinh không dừng được của người khác.
func (s *service) StopScreenShare(ctx context.Context, actorID string, classroomID, roomID uint, userID string) *httpx.HTTPError {
	room, isHost, err := s.liveRoomForUser(ctx, actorID, classroomID, roomID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	if actorID != userID {
		if !isHost {
			return ErrClassroomForbidden
		}
		if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
			return httpx.FromError(err, errClassroomServiceFailed)
		}
	}
	return httpx.FromError(s.stopScreenShare(ctx, room, userID), errClassroomServiceFailed)
}

// liveRoomForUser trả về room đang LIVE của lớp cùng cờ host; người dùng phải
// là giáo viên hoặc học sinh đã được duyệt vào lớp.
func (s *service) liveRoomForUser(ctx context.Context, userID string, classroomID, roomID uint) (*model.Room, bool, error) {
	room, err := s.repo.GetRoomByID(ctx, roomID)
	if err != nil {
		return nil, false, err
	}
	if room.Schedule.ClassroomID != classroomID {
		return nil, false, ErrClassroomForbidden
	}
	isHost := room.Schedule.Classroom.TeacherID == userID
	if !isHost {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, false, err
		}
		if !approved {
			return nil, false, ErrClassroomForbidden
		}
	}
	switch room.Status {
	case model.RoomStatusLive:
		return room, isHost, nil
	case model.RoomStatusEnded:
		return nil, false, ErrRoomEnded
	default:
		return nil, false, ErrRoomNotLive
	}
}

// ensureRoomStudent bảo đảm userID là học sinh đã được duyệt của lớp (không phải giáo viên),
// để giáo viên không thao tác nhầm lên chính mình hoặc lên người ngoài lớp.
func (s *service) ensureRoomStudent(ctx context.Context, room *model.Room, userID string) error {
	if userID == room.Schedule.Classroom.TeacherID {
		return ErrRoomParticipantInvalid
	}
	approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
	if err != nil {
		return err
	}
	if !approved {
		return ErrRoomParticipantInvalid
	}
	return nil
}

// mediaError đổi lỗi LiveKit sang lỗi nghiệp vụ: người không còn kết nối thành
// ErrParticipantNotConnected (409), còn lại là ErrMediaUnavailable; giữ lỗi gốc để log.
func mediaError(err error) error {
	if errors.Is(err, media.ErrParticipantNotFound) {
		return errors.Join(ErrParticipantNotConnected, err)
	}
	return errors.Join(ErrMediaUnavailable, err)
}

// HandleScreenSharePublished là lớp bảo vệ cuối khi client publish mà không
// qua StartScreenShare: học sinh chưa được duyệt bị dừng, lượt cũ bị thay thế.
func (s *service) HandleScreenSharePublished(ctx context.Context, roomName, userID string) error {
	room, err := s.roomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if room.Status != model.RoomStatusLive || s.mediaClient == nil {
		return nil
	}
	if userID != room.Schedule.Classroom.TeacherID {
		state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if state == nil || state.ScreenShareApprovedAt == nil {
			return s.stopScreenShare(ctx, room, userID)
		}
	}
	return s.stopOtherScreenShares(ctx, room, userID)
}

// HandleScreenShareUnpublished (webhook) dọn state khi track chia sẻ màn hình bị gỡ, kể cả khi
// client tự tắt mà không gọi API, để lần chia sẻ sau phải xin lại và quyền LiveKit được thu hồi.
func (s *service) HandleScreenShareUnpublished(ctx context.Context, roomName, userID string) error {
	room, err := s.roomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if room.Status != model.RoomStatusLive {
		return nil
	}
	changed, err := s.clearScreenShareState(ctx, room.ID, userID)
	if err != nil {
		return err
	}
	// Thu hồi sau khi đã xóa phê duyệt trong DB: quyền đồng bộ từ state mới nhất.
	if userID != room.Schedule.Classroom.TeacherID && s.mediaClient != nil {
		if err := s.syncParticipantPermissions(ctx, room, userID, false); err != nil {
			logger.Log.Warn().Err(err).Uint("room_id", room.ID).Str("user_id", userID).Msg("[ERROR] Failed to revoke screen share permission")
		}
	}
	if changed {
		s.publishRoomActivity(room, learningSocket.EventScreenShareStopped, userID)
	}
	return nil
}

// stopOtherScreenShares dừng mọi người đang chia sẻ màn hình trừ userID, vì mỗi phòng chỉ
// cho một người chia sẻ tại một thời điểm.
func (s *service) stopOtherScreenShares(ctx context.Context, room *model.Room, userID string) error {
	if s.mediaClient == nil {
		return ErrMediaUnavailable
	}
	sharers, err := s.mediaClient.ListScreenSharers(ctx, media.RoomName(room.ID))
	if err != nil {
		return mediaError(err)
	}
	for _, sharer := range sharers {
		if sharer == userID {
			continue
		}
		if err := s.stopScreenShare(ctx, room, sharer); err != nil {
			return err
		}
	}
	return nil
}

// stopScreenShare xóa yêu cầu/phê duyệt, dừng media, thu hồi quyền và báo cho người
// chia sẻ để client gỡ track; lần chia sẻ sau phải xin lại. DB đổi trước để quyền đồng bộ
// sang LiveKit tính từ state đã xóa phê duyệt.
func (s *service) stopScreenShare(ctx context.Context, room *model.Room, userID string) error {
	if s.mediaClient == nil {
		return ErrMediaUnavailable
	}
	if _, err := s.clearScreenShareState(ctx, room.ID, userID); err != nil {
		return err
	}
	if err := s.stopScreenShareMedia(ctx, room, userID); err != nil {
		return err
	}
	s.publishRoomActivity(room, learningSocket.EventScreenShareStopped, userID)
	return nil
}

// stopScreenShareMedia dừng track chia sẻ màn hình và đồng bộ quyền (giữ nguyên mic/camera
// hiện tại). Gọi sau khi DB đã xóa phê duyệt.
func (s *service) stopScreenShareMedia(ctx context.Context, room *model.Room, userID string) error {
	if s.mediaClient == nil {
		return ErrMediaUnavailable
	}
	// Participant đã rời phòng thì không còn track hay quyền để thu hồi.
	if err := s.mediaClient.MuteTracks(ctx, media.RoomName(room.ID), userID, media.SourceScreenShare); err != nil {
		if errors.Is(err, media.ErrParticipantNotFound) {
			return nil
		}
		return mediaError(err)
	}
	if userID == room.Schedule.Classroom.TeacherID {
		return nil
	}
	return s.syncParticipantPermissions(ctx, room, userID, false)
}
