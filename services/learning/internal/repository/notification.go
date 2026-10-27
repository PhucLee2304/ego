package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) CreateNotificationOutbox(ctx context.Context, event *model.NotificationOutbox) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_key"}}, DoNothing: true}).Create(event).Error
}

func (r *repository) ClaimNotificationOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error) {
	var events []*model.NotificationOutbox
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
		return tx.Model(&model.NotificationOutbox{}).Where("id IN ?", ids).
			Updates(map[string]any{"next_attempt_at": now.Add(time.Minute)}).Error
	})
	return events, err
}

func (r *repository) MarkNotificationOutboxProcessed(ctx context.Context, id uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"processed_at": at, "last_error": nil}).Error
}

func (r *repository) MarkNotificationOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error {
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).Where("id = ?", id).
		Updates(map[string]any{"attempts": attempts, "next_attempt_at": next, "last_error": message}).Error
}

func (r *repository) CancelNotificationOutbox(ctx context.Context, eventKeyPrefix string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&model.NotificationOutbox{}).
		Where("(event_key = ? OR event_key LIKE ?) AND processed_at IS NULL", eventKeyPrefix, eventKeyPrefix+":%").
		Updates(map[string]any{"processed_at": now, "last_error": "cancelled"}).Error
}
