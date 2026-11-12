package service

import (
	"context"
	"errors"
	"math"
	"time"

	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/logger"
)

// ProcessUserBlockedOutbox báo cho các service khác về những user vừa bị khoá, gọi từ worker.
// Mỗi event gọi lần lượt:
//   - notifications DeleteDeviceTokensByUserID: xoá token push, đóng WebSocket thông báo;
//   - learning RemoveUserFromLiveRooms: đá khỏi phòng live (kết thúc phòng nếu là giáo viên), đóng
//     WebSocket bài thi và lớp học.
//
// Chỉ đánh dấu xong khi cả hai rpc thành công; lỗi thì hẹn thử lại cả event (2^attempts giây, tối đa 5
// phút). Gọi lại an toàn vì cả hai rpc đều idempotent. Trả số event đã xử lý xong.
func (s *service) ProcessUserBlockedOutbox(ctx context.Context, limit int) (int, error) {
	if s.notificationsClient == nil || s.learningClient == nil {
		return 0, nil
	}
	events, err := s.repo.ClaimUserBlockedOutbox(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, event := range events {
		_, notifyErr := s.notificationsClient.DeleteDeviceTokensByUserID(ctx, &notificationsClient.DeleteDeviceTokensByUserIDRequest{UserId: event.UserID})
		_, learningErr := s.learningClient.RemoveUserFromLiveRooms(ctx, &learningClient.RemoveUserFromLiveRoomsRequest{UserId: event.UserID})
		if callErr := errors.Join(notifyErr, learningErr); callErr != nil {
			attempts := event.Attempts + 1
			if err := s.repo.MarkUserBlockedOutboxFailed(ctx, event.ID, attempts, time.Now().Add(userBlockedRetryDelay(attempts)), callErr.Error()); err != nil {
				return processed, err
			}
			logger.Log.Error().Err(callErr).Uint("event_id", event.ID).Str("user_id", event.UserID).Int("attempts", attempts).Msg("[USER_BLOCKED_OUTBOX] Publish failed")
			continue
		}
		if err := s.repo.MarkUserBlockedOutboxProcessed(ctx, event.ID, time.Now()); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// userBlockedRetryDelay: 2^attempts giây, tối đa 300 giây.
func userBlockedRetryDelay(attempts int) time.Duration {
	return time.Duration(math.Min(math.Pow(2, float64(attempts)), 300)) * time.Second
}
