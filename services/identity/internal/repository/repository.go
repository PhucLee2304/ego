package repository

import (
	"context"

	platformdb "ego/platform/db"
	"ego/services/identity/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	UpsertUser(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, userID string) (*model.User, error)
	GetByIDs(ctx context.Context, userIDs []string) ([]*model.User, error)
	SearchUserIDsByName(ctx context.Context, query string) ([]string, error)
	UpdateMe(ctx context.Context, userID string, updates map[string]any) (*model.User, error)
	GetList(ctx context.Context, limit, offset int32) ([]*model.User, int64, error)
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
