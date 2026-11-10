package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/services/posts/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) CreateOutbox(ctx context.Context, outbox *model.NotificationOutbox) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(outbox).Error
}

func (r *repository) ClaimOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error) {
	var outboxes []*model.NotificationOutbox
	err := platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND next_attempt_at <= ?", now).
			Order("id").
			Limit(limit).
			Find(&outboxes).Error; err != nil {
			return err
		}
		if len(outboxes) == 0 {
			return nil
		}
		ids := make([]uint, len(outboxes))
		for index, outbox := range outboxes {
			ids[index] = outbox.ID
		}
		return tx.Model(&model.NotificationOutbox{}).
			Where("id IN ?", ids).
			Update("next_attempt_at", now.Add(time.Minute)).Error
	})
	return outboxes, err
}

func (r *repository) MarkOutboxProcessed(ctx context.Context, id uint, processedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{"processed_at": processedAt, "last_error": nil}).Error
}

func (r *repository) MarkOutboxFailed(ctx context.Context, id uint, attempts int, nextAttemptAt time.Time, message string) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":        attempts,
			"next_attempt_at": nextAttemptAt,
			"last_error":      message,
		}).Error
}
