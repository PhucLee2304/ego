package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	// Notification outbox
	CreateNotificationOutbox(ctx context.Context, event *model.NotificationOutbox) error
	ClaimNotificationOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error)
	MarkNotificationOutboxProcessed(ctx context.Context, id uint, at time.Time) error
	MarkNotificationOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error
	CancelNotificationOutbox(ctx context.Context, eventKeyPrefix string) error

	Transaction(ctx context.Context, fn func(Repository) error) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}
