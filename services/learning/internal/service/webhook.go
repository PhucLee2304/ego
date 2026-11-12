package service

import (
	"context"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/logger"
	"ego/services/learning/internal/model"
	learningSocket "ego/services/learning/internal/socket"
)

// HandleParticipantJoined mở phiên điểm danh khi user vào phòng LiveKit.
// User đã bị admin khoá thì bị đá ra ngay và không mở phiên: token LiveKit (kể cả token LiveKit tự cấp
// lại cho người đang trong phòng) còn hạn sau khi bị khoá, nên đây là chốt chặn cuối khi user vào lại.
// Không hỏi được identity thì vẫn cho vào (chỉ ghi log) để lỗi identity không làm hỏng điểm danh.
func (s *service) HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if s.identityClient != nil {
		resp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{userID}})
		if err != nil {
			logger.Log.Error().Err(err).Str("user_id", userID).Msg("[LEARNING] Failed to check blocked state of joined participant")
		} else if len(resp.Users) == 1 && resp.Users[0].Blocked {
			return ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, roomName, userID))
		}
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
