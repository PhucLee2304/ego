package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ego/platform/logger"
	"ego/services/learning/internal/media"
)

// RemoveUserFromLiveRooms đưa user bị khoá ra khỏi mọi phòng LIVE, gọi từ rpc (identity SetBlocked).
// Token đăng nhập của user đã bị từ chối nên họ không xin được token LiveKit mới; việc ở đây là cắt
// kết nối LiveKit đang mở:
//   - user là giáo viên của phòng: kết thúc phòng như EndRoom (ENDED rồi xoá phòng LiveKit), vì phòng
//     không còn người điều khiển;
//   - user là học sinh: ngắt user khỏi phòng LiveKit; webhook participant_left đóng phiên điểm danh.
//
// Lỗi của từng phòng được gom lại và trả sau khi đã thử hết các phòng.
func (s *service) RemoveUserFromLiveRooms(ctx context.Context, userID string) (removed, ended int, err error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, 0, ErrInvalidUserID
	}
	rooms, err := s.repo.GetLiveRoomsByUser(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	var errs []error
	for _, room := range rooms {
		roomName := media.RoomName(room.ID)
		if room.Schedule.Classroom.TeacherID == userID {
			if err := s.finishRoom(ctx, room); err != nil {
				errs = append(errs, fmt.Errorf("finish room %d: %w", room.ID, err))
				continue
			}
			ended++
			if s.mediaClient != nil {
				if err := s.mediaClient.DeleteRoom(ctx, roomName); err != nil {
					errs = append(errs, fmt.Errorf("delete media room %d: %w", room.ID, err))
				}
			}
			continue
		}
		if s.mediaClient == nil {
			continue
		}
		if err := ignoreNotConnected(s.mediaClient.RemoveParticipant(ctx, roomName, userID)); err != nil {
			errs = append(errs, fmt.Errorf("remove from room %d: %w", room.ID, err))
			continue
		}
		removed++
	}
	if len(errs) > 0 {
		logger.Log.Error().Err(errors.Join(errs...)).Str("user_id", userID).Msg("[LEARNING] Failed to remove blocked user from some live rooms")
	}
	return removed, ended, errors.Join(errs...)
}
