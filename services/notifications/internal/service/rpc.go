package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ego/platform/logger"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
	"ego/services/notifications/internal/repository"

	"gorm.io/datatypes"
)

// CreateNotifications lưu một event thông báo cho nhiều người nhận, gọi từ rpc (outbox của posts và
// learning). Trả số thông báo thực sự được tạo:
//   - thiếu title/sourceService/idempotencyKey, người nhận rỗng hoặc quá MaxRecipients thì trả
//     ErrInvalidNotification (rpc map sang InvalidArgument);
//   - người nhận trùng nhau chỉ tạo một thông báo; key của mỗi người là "<key>:user:<id>";
//   - mọi bản ghi insert trong một transaction; key đã có (outbox gửi lại) thì bỏ qua, không đếm;
//   - sau khi commit: báo WebSocket NOTIFICATION_CREATED cho từng user có thông báo mới, rồi gửi push
//     FCM best-effort ở goroutine riêng.
func (s *service) CreateNotifications(ctx context.Context, req dto.CreateNotificationsRequest) (int, error) {
	baseKey := strings.TrimSpace(req.IdempotencyKey)
	title := strings.TrimSpace(req.Title)
	source := strings.TrimSpace(req.SourceService)
	if len(req.Recipients) == 0 || len(req.Recipients) > MaxRecipients {
		return 0, fmt.Errorf("%w: recipients must contain between 1 and %d users", ErrInvalidNotification, MaxRecipients)
	}
	if baseKey == "" || title == "" || source == "" {
		return 0, fmt.Errorf("%w: title, source service and idempotency key are required", ErrInvalidNotification)
	}

	seen := make(map[string]bool, len(req.Recipients))
	items := make([]*model.Notification, 0, len(req.Recipients))
	for _, recipient := range req.Recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			return 0, fmt.Errorf("%w: recipient is required", ErrInvalidNotification)
		}
		if seen[recipient] {
			continue
		}
		seen[recipient] = true
		data := datatypes.JSONMap{}
		for key, value := range req.Data {
			data[key] = value
		}
		items = append(items, &model.Notification{
			UserID:         recipient,
			Title:          title,
			Body:           strings.TrimSpace(req.Body),
			SourceService:  source,
			Data:           data,
			IdempotencyKey: fmt.Sprintf("%s:user:%s", baseKey, recipient),
		})
	}

	var created []*model.Notification
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		var err error
		created, err = repo.CreateNotifications(ctx, items)
		return err
	})
	if err != nil {
		return 0, err
	}

	recipients := make([]string, len(created))
	for i, item := range created {
		recipients[i] = item.UserID
		if s.broadcast != nil {
			s.broadcast(item.UserID, map[string]any{"type": NotificationCreatedEvent})
		}
	}
	// Mọi thông báo trong lô cùng một event (cùng title, body, data) nên gửi push một lần cho cả lô.
	if s.sender != nil && len(created) > 0 {
		go s.sendPush(context.WithoutCancel(ctx), recipients, created[0].Title, created[0].Body, req.Data)
	}
	return len(created), nil
}

// sendPush gửi FCM một lần tới mọi thiết bị của cả lô người nhận (một truy vấn token, một lần gọi
// sender) và xoá token FCM báo không còn hợp lệ.
// Chạy sau khi đã lưu DB nên lỗi chỉ ghi log; ctx không bị huỷ theo request gọi tới.
func (s *service) sendPush(ctx context.Context, recipients []string, title, body string, data map[string]string) {
	pushCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	tokens, err := s.repo.GetDeviceTokens(pushCtx, recipients)
	if err != nil {
		logger.Log.Error().Err(err).Int("recipients", len(recipients)).Msg("[NOTIFICATIONS] Failed to load device tokens")
		return
	}
	if len(tokens) == 0 {
		return
	}
	// Sender tự chia lô 500 token theo giới hạn multicast của FCM.
	invalid, err := s.sender.Send(pushCtx, tokens, title, body, data)
	if err != nil {
		logger.Log.Error().Err(err).Int("recipients", len(recipients)).Msg("[NOTIFICATIONS] FCM send failed")
	}
	if len(invalid) == 0 {
		return
	}
	// Cleanup dùng timeout riêng vì pushCtx có thể đã hết hạn sau khi gọi FCM.
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
	defer cleanupCancel()
	if err := s.repo.DeleteDeviceTokens(cleanupCtx, invalid); err != nil {
		logger.Log.Error().Err(err).Msg("[NOTIFICATIONS] Failed to delete invalid device tokens")
	}
}

// DeleteDeviceTokensByUserID xoá mọi token push của user để máy của họ không nhận push nữa, gọi từ rpc
// (identity, khi admin khoá tài khoản). userID rỗng trả ErrInvalidNotification. Khi được mở khoá,
// user đăng nhập lại thì client tự đăng ký token mới.
func (s *service) DeleteDeviceTokensByUserID(ctx context.Context, userID string) (int, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, fmt.Errorf("%w: user id is required", ErrInvalidNotification)
	}
	deleted, err := s.repo.DeleteDeviceTokensByUserID(ctx, userID)
	return int(deleted), err
}
