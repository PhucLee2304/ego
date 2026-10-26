package service

import (
	"context"
	"time"

	"ego/services/learning/internal/model"
	learningSocket "ego/services/learning/internal/socket"
)

func (s *service) HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	role := model.RoomParticipantRoleParticipant
	if userID == room.Schedule.Classroom.TeacherID {
		role = model.RoomParticipantRoleHost
		if err := s.repo.SetRoomHostAbsentSince(ctx, room.ID, nil); err != nil {
			return err
		}
	}
	return s.repo.OpenParticipantSession(ctx, room.ID, userID, role, joinedAt)
}

func (s *service) HandleParticipantLeft(ctx context.Context, roomName, userID string, leftAt time.Time) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if err := s.repo.CloseParticipantSession(ctx, room.ID, userID, leftAt); err != nil {
		return err
	}
	// Phê duyệt chia sẻ màn hình gắn với phiên đang kết nối; vào lại phòng
	// bằng token mới thì phải xin lại.
	if changed, err := s.clearScreenShareState(ctx, room.ID, userID); err != nil {
		return err
	} else if changed {
		s.publishRoomActivity(room, learningSocket.EventScreenShareStopped, userID)
	}
	if userID == room.Schedule.Classroom.TeacherID {
		return s.repo.SetRoomHostAbsentSince(ctx, room.ID, &leftAt)
	}
	return nil
}
