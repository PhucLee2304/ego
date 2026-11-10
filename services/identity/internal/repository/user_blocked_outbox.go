package repository

import (
	"context"
	"time"

	"ego/services/identity/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CreateUserBlockedOutbox ghi event "user bị khoá"; gọi trong transaction của SetBlocked.
func (r *repository) CreateUserBlockedOutbox(ctx context.Context, event *model.UserBlockedOutbox) error {
	return r.db.WithContext(ctx).Create(event).Error
}

// ClaimUserBlockedOutbox lấy tối đa limit event đến hạn và đẩy next_attempt_at lùi 1 phút, để nhiều
// worker (nhiều instance identity) không xử lý trùng: SKIP LOCKED bỏ qua dòng instance khác đang giữ,
// còn event bị claim mà worker chết giữa chừng sẽ được lấy lại sau 1 phút.
func (r *repository) ClaimUserBlockedOutbox(ctx context.Context, now time.Time, limit int) ([]*model.UserBlockedOutbox, error) {
	var events []*model.UserBlockedOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND next_attempt_at <= ?", now).
			Order("id ASC").Limit(limit).Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		ids := make([]uint, len(events))
		for i, event := range events {
			ids[i] = event.ID
		}
		return tx.Model(&model.UserBlockedOutbox{}).Where("id IN ?", ids).
			Update("next_attempt_at", now.Add(time.Minute)).Error
	})
	return events, err
}

// MarkUserBlockedOutboxProcessed đánh dấu event đã báo xong cho mọi service.
func (r *repository) MarkUserBlockedOutboxProcessed(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserBlockedOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"processed_at": at, "last_error": nil}).Error
}

// MarkUserBlockedOutboxFailed ghi lại lần thử lỗi và hẹn lần thử tiếp theo.
func (r *repository) MarkUserBlockedOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error {
	return r.db.WithContext(ctx).Model(&model.UserBlockedOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"attempts": attempts, "next_attempt_at": next, "last_error": message}).Error
}

// CancelUserBlockedOutbox huỷ các event chưa xử lý của user; gọi khi admin mở khoá để worker không xoá
// token hay đá khỏi phòng một user vừa được mở khoá.
func (r *repository) CancelUserBlockedOutbox(ctx context.Context, userID string, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.UserBlockedOutbox{}).
		Where("user_id = ? AND processed_at IS NULL", userID).
		Updates(map[string]any{"processed_at": at, "last_error": "cancelled"}).Error
}
