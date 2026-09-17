package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/services/classrooms/internal/model"

	"gorm.io/gorm"
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

func (r *Repository) DB() *gorm.DB {
	return r.db
}

func (r *Repository) GetAssignmentForAttemptValidation(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("id = ?", id).
		First(&assignment).Error; err != nil {
		return nil, err
	}

	return &assignment, nil
}

func (r *Repository) IsStudentApprovedInClassroom(ctx context.Context, classroomID uint, userID string) (bool, error) {
	var count int64
	approved := true
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND user_id = ? AND approved = ?", classroomID, userID, approved).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) UpsertAssignmentSubmission(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) UpsertAssignmentSubmissionResult(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
			"score":      submission.Score,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) GetAssignmentSubmissionByAssignmentAndStudent(ctx context.Context, assignmentID uint, studentID string) (*model.AssignmentSubmission, error) {
	var submission model.AssignmentSubmission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		First(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}
