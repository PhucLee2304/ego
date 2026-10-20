package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/media"
	"ego/services/classrooms/internal/model"
	classroomsSocket "ego/services/classrooms/internal/socket"
)

var (
	ErrScreenShareTeacherDirect = httpx.NewError(http.StatusBadRequest, "[ERROR] The teacher can share the screen directly without a request", nil)
	ErrScreenShareNotRequested  = httpx.NewError(http.StatusConflict, "[CONFLICT] This participant has no screen share request", nil)
	ErrScreenShareNotApproved   = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Screen sharing has not been approved by the teacher", nil)
	ErrRoomParticipantInvalid   = httpx.NewError(http.StatusBadRequest, "[ERROR] The user is not a student of this classroom", nil)
	ErrParticipantNotConnected  = httpx.NewError(http.StatusConflict, "[CONFLICT] The participant is not connected to the room", nil)
)

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

func mediaError(err error) error {
	if errors.Is(err, media.ErrParticipantNotFound) {
		return errors.Join(ErrParticipantNotConnected, err)
	}
	return errors.Join(ErrMediaUnavailable, err)
}

func (s *service) RequestScreenShare(ctx context.Context, userID string, classroomID, roomID uint) (*dto.RoomParticipantStateResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if isHost {
		return nil, ErrScreenShareTeacherDirect
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, err
	}
	if state.RemovedAt != nil {
		return nil, ErrParticipantRemoved
	}
	// Gửi lại khi đang chờ hoặc đã được duyệt không tạo yêu cầu mới.
	if state.ScreenShareRequestedAt == nil {
		now := time.Now()
		state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = &now, nil
		if err := s.repo.UpdateRoomParticipantState(ctx, state); err != nil {
			return nil, err
		}
		s.publishRoomActivity(room, classroomsSocket.EventScreenShareRequested, userID)
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

func (s *service) ApproveScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
		return nil, err
	}
	state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, err
	}
	if state == nil || state.ScreenShareRequestedAt == nil {
		return nil, ErrScreenShareNotRequested
	}
	if state.ScreenShareApprovedAt != nil {
		return dto.ToRoomParticipantStateResponse(state), nil
	}
	if s.mediaClient == nil {
		return nil, ErrMediaUnavailable
	}
	// Cấp quyền ở LiveKit trước khi lưu: học sinh chỉ publish được screen
	// share khi server cho phép, không dựa vào việc client tự kiểm tra.
	if err := s.mediaClient.SetPublishPermissions(ctx, media.RoomName(room.ID), userID, participantPermissions(state, true)); err != nil {
		return nil, mediaError(err)
	}
	now := time.Now()
	state.ScreenShareApprovedAt = &now
	if err := s.repo.UpdateRoomParticipantState(ctx, state); err != nil {
		return nil, err
	}
	s.publishRoomActivity(room, classroomsSocket.EventScreenShareApproved, userID)
	return dto.ToRoomParticipantStateResponse(state), nil
}

func (s *service) RejectScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
		return nil, err
	}
	state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, err
	}
	if state == nil || state.ScreenShareRequestedAt == nil {
		return nil, ErrScreenShareNotRequested
	}
	// Từ chối sau khi đã duyệt cũng thu hồi quyền và dừng phần đang chia sẻ.
	if state.ScreenShareApprovedAt != nil {
		if err := s.stopScreenShareMedia(ctx, room, userID); err != nil {
			return nil, err
		}
	}
	state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = nil, nil
	if err := s.repo.UpdateRoomParticipantState(ctx, state); err != nil {
		return nil, err
	}
	s.publishRoomActivity(room, classroomsSocket.EventScreenShareRejected, userID)
	return dto.ToRoomParticipantStateResponse(state), nil
}

// StartScreenShare được gọi ngay trước khi client publish screen share. Mỗi
// phòng chỉ có một người chia sẻ nên backend dừng lượt đang chạy trước.
func (s *service) StartScreenShare(ctx context.Context, userID string, classroomID, roomID uint) error {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return err
	}
	if !isHost {
		state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if state == nil || state.ScreenShareApprovedAt == nil {
			return ErrScreenShareNotApproved
		}
	}
	return s.stopOtherScreenShares(ctx, room, userID)
}

func (s *service) StopScreenShare(ctx context.Context, actorID string, classroomID, roomID uint, userID string) error {
	room, isHost, err := s.liveRoomForUser(ctx, actorID, classroomID, roomID)
	if err != nil {
		return err
	}
	if actorID != userID {
		if !isHost {
			return ErrClassroomForbidden
		}
		if err := s.ensureRoomStudent(ctx, room, userID); err != nil {
			return err
		}
	}
	return s.stopScreenShare(ctx, room, userID)
}

// HandleScreenSharePublished là lớp bảo vệ cuối khi client publish mà không
// qua StartScreenShare: học sinh chưa được duyệt bị dừng, lượt cũ bị thay thế.
func (s *service) HandleScreenSharePublished(ctx context.Context, roomName, userID string) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
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

func (s *service) HandleScreenShareUnpublished(ctx context.Context, roomName, userID string) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if room.Status != model.RoomStatusLive {
		return nil
	}
	if userID != room.Schedule.Classroom.TeacherID && s.mediaClient != nil {
		state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
		if err != nil {
			return err
		}
		if err := s.mediaClient.SetPublishPermissions(ctx, media.RoomName(room.ID), userID, participantPermissions(state, false)); err != nil && !errors.Is(err, media.ErrParticipantNotFound) {
			logger.Log.Warn().Err(err).Uint("room_id", room.ID).Str("user_id", userID).Msg("[ROOM] Failed to revoke screen share permission")
		}
	}
	changed, err := s.clearScreenShareState(ctx, room.ID, userID)
	if err != nil {
		return err
	}
	if changed {
		s.publishRoomActivity(room, classroomsSocket.EventScreenShareStopped, userID)
	}
	return nil
}

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

// stopScreenShare dừng media, thu hồi quyền, xóa yêu cầu/phê duyệt và báo cho
// người chia sẻ để client gỡ track; lần chia sẻ sau phải xin lại.
func (s *service) stopScreenShare(ctx context.Context, room *model.Room, userID string) error {
	if err := s.stopScreenShareMedia(ctx, room, userID); err != nil {
		return err
	}
	if _, err := s.clearScreenShareState(ctx, room.ID, userID); err != nil {
		return err
	}
	s.publishRoomActivity(room, classroomsSocket.EventScreenShareStopped, userID)
	return nil
}

func (s *service) stopScreenShareMedia(ctx context.Context, room *model.Room, userID string) error {
	if s.mediaClient == nil {
		return ErrMediaUnavailable
	}
	roomName := media.RoomName(room.ID)
	// Participant đã rời phòng thì không còn track hay quyền để thu hồi.
	if err := s.mediaClient.MuteTracks(ctx, roomName, userID, media.SourceScreenShare); err != nil {
		if errors.Is(err, media.ErrParticipantNotFound) {
			return nil
		}
		return mediaError(err)
	}
	if userID == room.Schedule.Classroom.TeacherID {
		return nil
	}
	// Thu hồi screen share nhưng giữ nguyên quyền mic/camera hiện tại.
	state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return err
	}
	if err := s.mediaClient.SetPublishPermissions(ctx, roomName, userID, participantPermissions(state, false)); err != nil && !errors.Is(err, media.ErrParticipantNotFound) {
		return mediaError(err)
	}
	return nil
}

func (s *service) clearScreenShareState(ctx context.Context, roomID uint, userID string) (bool, error) {
	state, err := s.repo.FindRoomParticipantState(ctx, roomID, userID)
	if err != nil || state == nil {
		return false, err
	}
	if state.ScreenShareRequestedAt == nil && state.ScreenShareApprovedAt == nil {
		return false, nil
	}
	state.ScreenShareRequestedAt, state.ScreenShareApprovedAt = nil, nil
	return true, s.repo.UpdateRoomParticipantState(ctx, state)
}

// publishRoomActivity chỉ gửi cho giáo viên và người liên quan vì học sinh chỉ
// được xem state của chính mình.
func (s *service) publishRoomActivity(room *model.Room, eventType, userID string) {
	if s.roomPublisher == nil {
		return
	}
	event := classroomsSocket.NewRoomEvent(eventType, room.ID, classroomsSocket.ParticipantActivityPayload{UserID: userID})
	teacherID := room.Schedule.Classroom.TeacherID
	s.roomPublisher(teacherID, event)
	if userID != teacherID {
		s.roomPublisher(userID, event)
	}
}
