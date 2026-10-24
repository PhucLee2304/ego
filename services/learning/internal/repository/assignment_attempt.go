package repository

import (
	"context"
	"errors"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

// Truy vấn attempt theo assignment (context CLASSROOM_ASSIGNMENT).

func (r *repository) GetActiveAttemptsByAssignmentID(ctx context.Context, assignmentID uint) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Where("context_type = ? AND context_id = ? AND status = ?", model.AttemptContextTypeClassroomAssignment, assignmentID, model.AttemptStatusInProgress).
		Order("id ASC").
		Find(&attempts).Error; err != nil {
		return nil, err
	}
	return attempts, nil
}

func (r *repository) ExpireActiveAssignmentAttempts(ctx context.Context, assignmentID uint, now time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Attempt{}).
		Where("context_type = ? AND context_id = ? AND status = ?", model.AttemptContextTypeClassroomAssignment, assignmentID, model.AttemptStatusInProgress).
		Update("expires_at", now).Error
}

func (r *repository) CountAssignmentAttempts(ctx context.Context, assignmentID uint) (int64, int64, error) {
	var total, active int64
	baseConditions := "context_type = ? AND context_id = ?"
	if err := r.db.WithContext(ctx).Model(&model.Attempt{}).
		Where(baseConditions, model.AttemptContextTypeClassroomAssignment, assignmentID).
		Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if err := r.db.WithContext(ctx).Model(&model.Attempt{}).
		Where(baseConditions, model.AttemptContextTypeClassroomAssignment, assignmentID).
		Where("status = ?", model.AttemptStatusInProgress).
		Count(&active).Error; err != nil {
		return 0, 0, err
	}
	return active, total, nil
}

func (r *repository) UpdateActiveAssignmentAttemptSchedule(ctx context.Context, assignmentID uint, duration int, dueAt time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&model.Attempt{}).
		Where("context_type = ? AND context_id = ? AND status = ?", model.AttemptContextTypeClassroomAssignment, assignmentID, model.AttemptStatusInProgress).
		Updates(map[string]any{
			"duration":   duration,
			"expires_at": gorm.Expr("LEAST(started_at + (? * interval '1 minute'), ?)", duration, dueAt),
		})
	return result.RowsAffected, result.Error
}

func (r *repository) ResetAssignmentAttempt(ctx context.Context, assignmentID uint, studentID string) error {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).Where("context_type = ? AND context_id = ? AND user_id = ?", model.AttemptContextTypeClassroomAssignment, assignmentID, studentID).First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := r.db.WithContext(ctx).Where("attempt_id = ?", attempt.ID).Delete(&model.AttemptAnswer{}).Error; err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("attempt_id = ?", attempt.ID).Delete(&model.History{}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Delete(&attempt).Error
}
