package repository

import (
	"context"
	"ego/services/exams/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetList(ctx context.Context, limit, offset int32) ([]*model.Exam, int64, error) {
	var exams []*model.Exam
	var total int64

	if err := r.db.WithContext(ctx).Model(&model.Exam{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&exams).Error; err != nil {
		return nil, 0, err
	}

	return exams, total, nil
}
