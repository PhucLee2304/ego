package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/valuex"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

// Nối attempt ↔ assignment submission, thay 3 RPC exams→classrooms
// (ValidateAssignmentAttempt, CreateAssignmentSubmission, UpdateAssignmentSubmissionResult).
// Submission được ghi trong tx của attempt nên hai bên commit/rollback cùng nhau;
// GRADEBOOK_UPDATED do caller gửi sau khi commit.

// validateAssignmentAttempt kiểm tra học sinh được làm assignment này với đề examID.
func (s *service) validateAssignmentAttempt(ctx context.Context, repo repository.Repository, userID string, assignmentID uint, examID uint) (*model.Assignment, error) {
	assignment, err := repo.GetAssignmentForAttemptValidation(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}

	if assignment.ExamID != examID {
		return nil, ErrAssignmentExamMismatch
	}
	if !assignment.Classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !assignment.Active {
		return nil, ErrAssignmentNotActive
	}

	isApproved, err := repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, userID)
	if err != nil {
		return nil, err
	}
	if !isApproved {
		return nil, ErrStudentNotApproved
	}

	now := time.Now()
	if now.Before(assignment.OpensAt) {
		return nil, ErrAssignmentNotOpen
	}
	if now.After(assignment.DueAt) {
		return nil, ErrAssignmentClosed
	}
	return assignment, nil
}

// createAssignmentSubmission gắn attempt vừa tạo vào submission của học sinh (khóa
// assignment rồi kiểm tra lại trạng thái lớp/assignment/thành viên).
func createAssignmentSubmission(ctx context.Context, repo repository.Repository, studentID string, assignmentID uint, attemptID uint) (*gradebookEvent, error) {
	assignment, err := repo.GetAssignmentForAttemptValidationForUpdate(ctx, assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}
	if !assignment.Classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !assignment.Active {
		return nil, ErrAssignmentNotActive
	}
	isApproved, err := repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, studentID)
	if err != nil {
		return nil, err
	}
	if !isApproved {
		return nil, ErrStudentNotApproved
	}
	submission := &model.AssignmentSubmission{AssignmentID: assignment.ID, StudentID: studentID, AttemptID: attemptID}
	if err := repo.UpsertAssignmentSubmission(ctx, submission); err != nil {
		return nil, err
	}
	return gradebookUpdatedEvent(assignment, submission, model.Status(submission, time.Now(), assignment.DueAt)), nil
}

// updateAssignmentSubmissionResult ghi điểm (0–10, làm tròn 2 chữ số) của attempt
// đã nộp vào submission. Không kiểm tra lớp/assignment còn hoạt động: bài nộp lúc
// tắt assignment hoặc xóa học sinh vẫn phải có điểm.
func updateAssignmentSubmissionResult(ctx context.Context, repo repository.Repository, attempt *model.Attempt) (*gradebookEvent, error) {
	if *attempt.Score < 0 || *attempt.Score > 10 {
		return nil, ErrInvalidSubmissionScore
	}
	score := valuex.RoundFloat(*attempt.Score, 2)

	assignment, err := repo.GetAssignmentForAttemptValidation(ctx, *attempt.ContextID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}
	submission := &model.AssignmentSubmission{
		AssignmentID: assignment.ID,
		StudentID:    attempt.UserID,
		AttemptID:    attempt.ID,
		Score:        &score,
	}
	if err := repo.UpsertAssignmentSubmissionResult(ctx, submission); err != nil {
		return nil, err
	}
	return gradebookUpdatedEvent(assignment, submission, model.SubmissionStatusSubmitted), nil
}

// gradebookEvent là một GRADEBOOK_UPDATED gửi cho giáo viên của lớp.
type gradebookEvent struct {
	teacherID string
	event     socket.GradebookUpdatedEvent
}

func gradebookUpdatedEvent(assignment *model.Assignment, submission *model.AssignmentSubmission, status model.SubmissionStatus) *gradebookEvent {
	return &gradebookEvent{teacherID: assignment.Classroom.TeacherID, event: socket.GradebookUpdatedEvent{
		Type: socket.EventGradebookUpdated,
		Payload: socket.GradebookUpdatedPayload{
			ClassroomID:  assignment.ClassroomID,
			AssignmentID: assignment.ID,
			StudentID:    submission.StudentID,
			AttemptID:    submission.AttemptID,
			Status:       string(status),
			Score:        submission.Score,
			UpdatedAt:    time.Now(),
		},
	}}
}

// publishGradebookEvents chỉ gọi sau khi transaction ghi submission commit.
func (s *service) publishGradebookEvents(events []*gradebookEvent) {
	if s.roomPublisher == nil {
		return
	}
	for _, event := range events {
		s.roomPublisher(event.teacherID, event.event)
	}
}

func assignmentAttemptExpiresAt(startedAt time.Time, durationMinutes int, dueAt time.Time) time.Time {
	expiresAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
	if dueAt.Before(expiresAt) {
		return dueAt
	}
	return expiresAt
}
