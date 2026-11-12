package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	UpsertUser(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, userID string) (*model.User, error)
	GetByIDForUpdate(ctx context.Context, userID string) (*model.User, error)
	GetByIDs(ctx context.Context, userIDs []string) ([]*model.User, error)
	SearchUserIDsByName(ctx context.Context, query string) ([]string, error)
	UpdateMe(ctx context.Context, userID string, updates map[string]any) (*model.User, error)
	GetList(ctx context.Context, query dto.UsersQuery) ([]*model.User, int64, error)
	GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error)

	// User blocked outbox
	CreateUserBlockedOutbox(ctx context.Context, event *model.UserBlockedOutbox) error
	ClaimUserBlockedOutbox(ctx context.Context, now time.Time, limit int) ([]*model.UserBlockedOutbox, error)
	MarkUserBlockedOutboxProcessed(ctx context.Context, id uint, at time.Time) error
	MarkUserBlockedOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error
	CancelUserBlockedOutbox(ctx context.Context, userID string, at time.Time) error

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
