package service

import (
	"context"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/services/classrooms/internal/dto"
	classroomsSocket "ego/services/classrooms/internal/socket"
)

var ErrHandTeacher = httpx.NewError(http.StatusBadRequest, "[ERROR] The teacher does not raise a hand in their own class", nil)

// SetHandRaised lưu trạng thái giơ tay trong suốt thời gian room LIVE để
// refresh trang không làm mất tay; state bị xóa khi room kết thúc.
func (s *service) SetHandRaised(ctx context.Context, userID string, classroomID, roomID uint, raised bool) (*dto.RoomParticipantStateResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if isHost {
		return nil, ErrHandTeacher
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, err
	}
	if state.RemovedAt != nil {
		return nil, ErrClassroomForbidden
	}
	// Gửi lại cùng trạng thái giữ nguyên thời điểm giơ tay ban đầu để thứ tự
	// phát biểu không bị đẩy xuống cuối.
	switch {
	case raised && state.HandRaisedAt == nil:
		now := time.Now()
		state.HandRaisedAt = &now
		if err := s.repo.UpdateRoomParticipantState(ctx, state); err != nil {
			return nil, err
		}
		s.publishRoomActivity(room, classroomsSocket.EventHandRaised, userID)
	case !raised && state.HandRaisedAt != nil:
		state.HandRaisedAt = nil
		if err := s.repo.UpdateRoomParticipantState(ctx, state); err != nil {
			return nil, err
		}
		s.publishRoomActivity(room, classroomsSocket.EventHandLowered, userID)
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

// LowerHands cho giáo viên hạ tay một học sinh ([userID] khác nil) hoặc toàn
// bộ học sinh đang giơ tay.
func (s *service) LowerHands(ctx context.Context, teacherID string, classroomID, roomID uint, userID *string) (*dto.LoweredHandsResponse, error) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, err
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if userID != nil {
		if err := s.ensureRoomStudent(ctx, room, *userID); err != nil {
			return nil, err
		}
	}
	lowered, err := s.repo.LowerRoomHands(ctx, room.ID, userID)
	if err != nil {
		return nil, err
	}
	for _, id := range lowered {
		s.publishRoomActivity(room, classroomsSocket.EventHandLowered, id)
	}
	return &dto.LoweredHandsResponse{UserIDs: lowered}, nil
}
