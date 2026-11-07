package repository

import (
	"context"
	"errors"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"

	"gorm.io/gorm"
)

// ErrConcurrentInsert: một lần gọi khác cùng event đã thêm một phần thông báo trong lúc đang insert;
// transaction phải rollback để bên gọi thử lại.
var ErrConcurrentInsert = errors.New("notifications inserted concurrently")

type Repository interface {
	// Notification
	GetNotifications(ctx context.Context, userID string, read *bool, types []string, limit, offset int) ([]*model.Notification, int64, int64, error)
	CountUnreadByType(ctx context.Context, userID string) (map[string]int64, error)
	MarkRead(ctx context.Context, userID string, ids []uint) error
	DeleteNotifications(ctx context.Context, userID string, ids []uint) error
	CreateNotifications(ctx context.Context, notifications []*model.Notification) ([]*model.Notification, error)

	// Device token
	UpsertDeviceToken(ctx context.Context, token *model.DeviceToken) error
	DeleteDeviceToken(ctx context.Context, userID, token string) error
	DeleteDeviceTokens(ctx context.Context, tokens []string) error
	GetDeviceTokens(ctx context.Context, userIDs []string) ([]string, error)
	DeleteDeviceTokensByUserID(ctx context.Context, userID string) (int64, error)

	// Stats (admin)
	GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, []StatsTypeCount, error)

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
