package repository

import (
	"context"

	"ego/services/learning/internal/model"
)

func (r *repository) GetAssignmentSubmissionByAssignmentAndStudent(ctx context.Context, assignmentID uint, studentID string) (*model.AssignmentSubmission, error) {
	var submission model.AssignmentSubmission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		First(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}

func (r *repository) DeleteAssignmentSubmission(ctx context.Context, assignmentID uint, studentID string) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		Delete(&model.AssignmentSubmission{}).Error
}
