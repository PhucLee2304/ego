package repository

import (
	"context"

	"ego/services/learning/internal/model"

	"gorm.io/gorm/clause"
)

func (r *repository) GetAttemptHistoryByIDAndUserID(ctx context.Context, id uint, userID string) (*model.History, error) {
	var history model.History
	if err := r.db.WithContext(ctx).
		Joins("JOIN attempts ON attempts.id = histories.attempt_id AND attempts.deleted_at IS NULL").
		Where("histories.attempt_id = ? AND attempts.user_id = ?", id, userID).
		First(&history).Error; err != nil {
		return nil, err
	}

	return &history, nil
}

func (r *repository) UpsertHistory(ctx context.Context, history *model.History) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "attempt_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"data",
			"exam_title",
			"exam_type",
			"exam_year",
			"updated_at",
			"deleted_at",
		}),
	}).Create(history).Error
}
