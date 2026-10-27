package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) GetActiveAttemptByUserID(ctx context.Context, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.AttemptStatusInProgress).
		Order("id DESC").
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *repository) GetAttemptsByUserIDAndStatus(ctx context.Context, userID string, status model.AttemptStatus, limit, offset int32) ([]*model.Attempt, int64, error) {
	var attempts []*model.Attempt
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Attempt{}).
		Where("user_id = ? AND status = ?", userID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Preload("Exam").
		Preload("Answers").
		Preload("History").
		Where("user_id = ? AND status = ?", userID, status).
		Order("id DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&attempts).Error; err != nil {
		return nil, 0, err
	}

	return attempts, total, nil
}

func (r *repository) GetAttemptQuestionsByIDAndUserID(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Preload("Answers").
		Preload("Exam.Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Where("group_id IS NULL").Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Where("id = ? AND user_id = ?", id, userID).
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *repository) GetAttemptForLifecycle(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	query := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Answers").
		Preload("History").
		Preload("Exam.Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Where("group_id IS NULL").Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Where("id = ? AND user_id = ?", id, userID)
	if err := query.First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *repository) CreateAttempt(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *repository) UpdateAttempt(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).
		Model(attempt).
		Select("status", "submitted_at", "correct_answers", "raw_score", "score", "updated_at").
		Updates(attempt).Error
}

// FinalizeAttempt ghi kết quả chốt chỉ khi attempt còn ACTIVE; false nếu đã bị
// transaction khác chốt trước.
func (r *repository) FinalizeAttempt(ctx context.Context, attempt *model.Attempt) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.Attempt{}).
		Where("id = ? AND status = ?", attempt.ID, model.AttemptStatusInProgress).
		Select("status", "submitted_at", "correct_answers", "raw_score", "score",
			"integrity_outcome", "terminated_at", "termination_reason", "terminated_by", "updated_at").
		Updates(attempt)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// GetActiveListeningOnlyAttempts lấy attempt TOEIC_STRICT đang làm mà layout chỉ gồm
// trang nghe (vd luyện Part 1), kèm các field để service tính lazy advance.
func (r *repository) GetActiveListeningOnlyAttempts(ctx context.Context, limit int) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Select("id", "user_id", "flow", "stage", "layout", "started_at", "listening_page", "listening_page_started_at").
		Where("status = ? AND flow = ?", model.AttemptStatusInProgress, model.AttemptFlowTOEICStrict).
		Where("NOT EXISTS (SELECT 1 FROM jsonb_array_elements(layout) page WHERE page->>'section' <> ?)", model.SectionCodeListening).
		Order("id ASC").
		Limit(limit).
		Find(&attempts).Error; err != nil {
		return nil, err
	}
	return attempts, nil
}

func (r *repository) GetExpiredActiveAttempts(ctx context.Context, now time.Time, limit int) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Select("id", "user_id").
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", model.AttemptStatusInProgress, now).
		Order("expires_at ASC").
		Limit(limit).
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}
