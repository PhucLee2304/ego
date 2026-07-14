package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/services/exams/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(repo *Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func (r *Repository) GetList(ctx context.Context, examType model.ExamType, limit, offset int32) ([]*model.Exam, int64, error) {
	var exams []*model.Exam
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Exam{}).Where("type = ?", examType)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&exams).Error; err != nil {
		return nil, 0, err
	}

	return exams, total, nil
}

func (r *Repository) GetByID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *Repository) GetQuestionsByExamID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *Repository) GetAttemptExamByID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *Repository) GetActiveAttemptByUserID(ctx context.Context, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.AttemptStatusInProgress).
		Order("id DESC").
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) CreateAttempt(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}
