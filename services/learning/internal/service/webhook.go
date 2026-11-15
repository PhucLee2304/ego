package service

import (
	"context"
	"errors"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/logger"
	"ego/services/learning/internal/model"
	learningSocket "ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

// HandleParticipantJoined mở phiên điểm danh khi user vào phòng LiveKit.
// User đã bị admin khoá thì bị đá ra ngay và không mở phiên: token LiveKit (kể cả token LiveKit tự cấp
// lại cho người đang trong phòng) còn hạn sau khi bị khoá, nên đây là chốt chặn cuối khi user vào lại.
// Không hỏi được identity thì vẫn cho vào (chỉ ghi log) để lỗi identity không làm hỏng điểm danh.
func (s *service) HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error {
	room, err := s.roomByName(ctx, roomName)
	if err != nil {
		return err
	}
	if s.identityClient != nil {
		resp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: []string{userID}})
		if err != nil {
			logger.Log.Error().Err(err).Str("user_id", userID).Msg("[ERROR] Failed to check blocked state of joined participant")
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
	} else {
		allowed, err := s.studentMayStayInRoom(ctx, room, userID)
		if err != nil {
			return err
		}
		if !allowed {
			return ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, roomName, userID))
		}
	}
	if err := s.repo.OpenParticipantSession(ctx, room.ID, userID, role, joinedAt); err != nil {
		return err
	}
	// Join token mang quyền lúc bấm vào phòng; giáo viên tắt mic/camera trước khi học sinh
	// kết nối xong thì lần đồng bộ đó bị bỏ qua (chưa có participant). Đồng bộ lại khi vào.
	if role == model.RoomParticipantRoleParticipant && s.mediaClient != nil {
		if err := s.syncParticipantPermissions(ctx, room, userID, false); err != nil {
			logger.Log.Warn().Err(err).Uint("room_id", room.ID).Str("user_id", userID).Msg("[ERROR] Failed to sync permissions of joined participant")
		}
	}
	return nil
}

// HandleParticipantLeft (webhook) đóng phiên điểm danh khi user rời phòng LiveKit. Giáo viên rời
// thì ghi HostAbsentSince để worker tự kết thúc phòng nếu vắng quá lâu sau giờ học.
func (s *service) HandleParticipantLeft(ctx context.Context, roomName, userID string, leftAt time.Time) error {
	room, err := s.roomByName(ctx, roomName)
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

// roomByName tìm phòng theo tên LiveKit; không có thì trả ErrRoomNotFound (webhook bỏ qua).
func (s *service) roomByName(ctx context.Context, roomName string) (*model.Room, error) {
	room, err := s.repo.GetRoomByName(ctx, roomName)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRoomNotFound
	}
	return room, err
}

// studentMayStayInRoom: học sinh còn được duyệt trong lớp và chưa bị mời ra khỏi phòng. Cùng
// lý do với user bị khoá, token LiveKit còn hạn sau khi bị mời ra/xóa khỏi lớp nên client
// reconnect là vào lại được; webhook join và worker đối chiếu đá ra, không mở phiên.
func (s *service) studentMayStayInRoom(ctx context.Context, room *model.Room, userID string) (bool, error) {
	approved, err := s.repo.IsStudentApprovedInClassroom(ctx, room.Schedule.ClassroomID, userID)
	if err != nil || !approved {
		return false, err
	}
	state, err := s.repo.FindRoomParticipantState(ctx, room.ID, userID)
	if err != nil {
		return false, err
	}
	return state == nil || state.RemovedAt == nil, nil
}
