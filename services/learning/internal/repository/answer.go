package repository

import (
	"context"

	"ego/services/learning/internal/model"

	"gorm.io/gorm/clause"
)

func (r *repository) GetAttemptForAnswerUpdate(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Exam.Sections.Groups.Questions.Options").
		Preload("Exam.Sections.Questions", "group_id IS NULL").
		Preload("Exam.Sections.Questions.Options").
		Where("id = ? AND user_id = ?", id, userID).
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *repository) UpsertAttemptAnswer(ctx context.Context, answer *model.AttemptAnswer) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "attempt_id"},
			{Name: "question_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"selected_option_id",
			"is_correct",
			"answered_at",
			"updated_at",
			"deleted_at",
		}),
	}).Create(answer).Error
}

func (r *repository) DeleteAttemptAnswer(ctx context.Context, attemptID, questionID uint) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
		Delete(&model.AttemptAnswer{}).Error
}

func (r *repository) ReplaceAttemptAnswers(ctx context.Context, attemptID uint, answers []model.AttemptAnswer) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("attempt_id = ?", attemptID).
		Delete(&model.AttemptAnswer{}).Error; err != nil {
		return err
	}

	if len(answers) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&answers).Error
}
