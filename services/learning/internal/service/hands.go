package service

import (
	"context"
	"net/http"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	learningSocket "ego/services/learning/internal/socket"
)

// ErrHandTeacher: giáo viên là người gọi phát biểu nên không có trạng thái giơ tay.
var ErrHandTeacher = httpx.NewError(http.StatusBadRequest, "[ERROR] The teacher does not raise a hand in their own class", nil)

// SetHandRaised lưu trạng thái giơ tay trong suốt thời gian room LIVE để
// refresh trang không làm mất tay; state bị xóa khi room kết thúc.
func (s *service) SetHandRaised(ctx context.Context, userID string, classroomID, roomID uint, raised bool) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, userID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if isHost {
		return nil, ErrHandTeacher
	}
	state, err := s.repo.GetOrCreateRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if state.RemovedAt != nil {
		return nil, ErrClassroomForbidden
	}
	// UPDATE có điều kiện thay cho đọc-rồi-ghi: hai request cùng lúc (bấm hai lần, giáo
	// viên hạ tay đồng thời) chỉ một bên đổi được và chỉ bên đó push. Gửi lại cùng trạng
	// thái giữ nguyên thời điểm giơ tay ban đầu để thứ tự phát biểu không bị đẩy xuống cuối.
	if raised {
		changed, err := s.repo.RaiseRoomHand(ctx, room.ID, userID, time.Now())
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if changed {
			s.publishRoomActivity(room, learningSocket.EventHandRaised, userID)
		}
	} else {
		lowered, err := s.repo.LowerRoomHands(ctx, room.ID, &userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if len(lowered) > 0 {
			s.publishRoomActivity(room, learningSocket.EventHandLowered, userID)
		}
	}
	// Dòng state chắc chắn đã có (GetOrCreate ở trên): chỉ cần đọc lại.
	if state, err = s.repo.FindRoomParticipantState(ctx, room.ID, userID); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	// Bị mời ra khỏi phòng đúng lúc giơ tay: UPDATE không đổi gì, trả Forbidden như khi đã bị mời ra.
	if state.RemovedAt != nil {
		return nil, ErrClassroomForbidden
	}
	return dto.ToRoomParticipantStateResponse(state), nil
}

// LowerHands cho giáo viên hạ tay một học sinh ([userID] khác nil) hoặc toàn
// bộ học sinh đang giơ tay.
func (s *service) LowerHands(ctx context.Context, teacherID string, classroomID, roomID uint, userID *string) (*dto.LoweredHandsResponse, *httpx.HTTPError) {
	room, isHost, err := s.liveRoomForUser(ctx, teacherID, classroomID, roomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if !isHost {
		return nil, ErrClassroomForbidden
	}
	if userID != nil {
		if err := s.ensureRoomStudent(ctx, room, *userID); err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
	}
	lowered, err := s.repo.LowerRoomHands(ctx, room.ID, userID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	for _, id := range lowered {
		s.publishRoomActivity(room, learningSocket.EventHandLowered, id)
	}
	return &dto.LoweredHandsResponse{UserIDs: lowered}, nil
}
