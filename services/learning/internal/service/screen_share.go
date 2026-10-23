package service

import (
	"context"

	"ego/services/learning/internal/model"
	learningSocket "ego/services/learning/internal/socket"
)

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
	event := learningSocket.NewRoomEvent(eventType, room.ID, learningSocket.ParticipantActivityPayload{UserID: userID})
	teacherID := room.Schedule.Classroom.TeacherID
	s.roomPublisher(teacherID, event)
	if userID != teacherID {
		s.roomPublisher(userID, event)
	}
}
