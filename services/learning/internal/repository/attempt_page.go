package repository

import (
	"context"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetAttemptForFlowUpdate khóa dòng attempt (FOR UPDATE) cho navigator/page: chỉ preload
// Answers, vì mọi kiểm tra của luồng strict đọc từ Layout (PLAN_TOEIC_ATTEMPT.md mục 1.2).
func (r *repository) GetAttemptForFlowUpdate(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Answers").
		Where("id = ? AND user_id = ?", id, userID).
		First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *repository) UpdateAttemptListening(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).
		Model(attempt).
		Select("stage", "listening_page", "listening_page_started_at", "updated_at").
		Updates(attempt).Error
}

// GetQuestionNumbers trả số câu gốc (Question.Number) theo ID; câu không có số thì vắng trong map.
func (r *repository) GetQuestionNumbers(ctx context.Context, ids []uint) (map[uint]int, error) {
	var rows []model.Question
	if err := r.db.WithContext(ctx).
		Select("id", "number").
		Where("id IN ? AND number IS NOT NULL", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	numbers := make(map[uint]int, len(rows))
	for _, row := range rows {
		numbers[row.ID] = *row.Number
	}
	return numbers, nil
}

// GetPageQuestions load các câu của một trang kèm option, đúng thứ tự trong ids.
func (r *repository) GetPageQuestions(ctx context.Context, ids []uint) ([]*model.Question, error) {
	var questions []*model.Question
	if err := r.db.WithContext(ctx).
		Preload("Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}})
		}).
		Where("id IN ?", ids).
		Find(&questions).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]*model.Question, len(questions))
	for _, question := range questions {
		byID[question.ID] = question
	}
	ordered := make([]*model.Question, 0, len(ids))
	for _, id := range ids {
		if question, ok := byID[id]; ok {
			ordered = append(ordered, question)
		}
	}
	return ordered, nil
}

func (r *repository) GetGroupByID(ctx context.Context, id uint) (*model.Group, error) {
	var group model.Group
	if err := r.db.WithContext(ctx).First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}
