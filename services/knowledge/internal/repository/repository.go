package repository

import (
	"context"

	platformdb "ego/platform/db"

	"gorm.io/gorm"
)

// Repository gom truy vấn của mọi domain trong knowledge (topic, dictionary, grammar).
// Method của từng domain được nhóm bằng comment `// Domain` và cài đặt ở <domain>.go.
type Repository interface {
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
