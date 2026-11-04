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

// DeleteAssignmentSubmission xóa hẳn (không xóa mềm): unique (assignment_id,
// student_id) gồm cả dòng đã xóa mềm, nên xóa mềm thì học sinh không làm lại được sau reset.
func (r *repository) DeleteAssignmentSubmission(ctx context.Context, assignmentID uint, studentID string) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		Delete(&model.AssignmentSubmission{}).Error
}

func (r *repository) UpsertAssignmentSubmission(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
		}).
		FirstOrCreate(submission).Error
}

func (r *repository) UpsertAssignmentSubmissionResult(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
			"score":      submission.Score,
		}).
		FirstOrCreate(submission).Error
}
