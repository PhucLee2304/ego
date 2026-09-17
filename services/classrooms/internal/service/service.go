package service

import (
	"context"
	"ego/platform/valuex"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Service interface {
	ValidateAssignmentAttempt(ctx context.Context, userID string, assignmentID uint, examID uint) (*dto.ValidateAssignmentAttemptResponse, error)
	CreateAssignmentSubmission(ctx context.Context, studentID string, assignmentID uint, attemptID uint) (*dto.AssignmentSubmissionResponse, error)
	UpdateAssignmentSubmissionResult(ctx context.Context, studentID string, assignmentID uint, attemptID uint, score float64) (*dto.AssignmentSubmissionResponse, error)
	GetAssignmentSubmissionSyncStatus(ctx context.Context, studentID string, assignmentID uint, attemptID uint, score float64) (*dto.AssignmentSubmissionSyncStatusResponse, error)
}

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

var (
	ErrAssignmentExamMismatch = errors.New("[ERROR] Assignment does not belong to exam")
	ErrAssignmentNotOpen      = errors.New("[ERROR] Assignment is not open yet")
	ErrAssignmentClosed       = errors.New("[ERROR] Assignment is closed")
	ErrStudentNotApproved     = errors.New("[FORBIDDEN] Student is not approved in classroom")
	ErrClassroomNotActive     = errors.New("[ERROR] Classroom is not active")
	ErrInvalidSubmissionScore = errors.New("[ERROR] Invalid submission score")
)

func (s *service) ValidateAssignmentAttempt(ctx context.Context, userID string, assignmentID uint, examID uint) (*dto.ValidateAssignmentAttemptResponse, error) {
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, assignmentID)
	if err != nil {
		return nil, err
	}

	if assignment.ExamID != examID {
		return nil, ErrAssignmentExamMismatch
	}
	if !assignment.Classroom.Active {
		return nil, ErrClassroomNotActive
	}

	isApproved, err := s.repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, userID)
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

	return &dto.ValidateAssignmentAttemptResponse{
		AssignmentID: assignment.ID,
		ClassroomID:  assignment.ClassroomID,
		ExamID:       assignment.ExamID,
	}, nil
}

func (s *service) CreateAssignmentSubmission(ctx context.Context, studentID string, assignmentID uint, attemptID uint) (*dto.AssignmentSubmissionResponse, error) {
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, assignmentID)
	if err != nil {
		return nil, err
	}
	if !assignment.Classroom.Active {
		return nil, ErrClassroomNotActive
	}

	isApproved, err := s.repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, studentID)
	if err != nil {
		return nil, err
	}
	if !isApproved {
		return nil, ErrStudentNotApproved
	}

	submission := &model.AssignmentSubmission{
		AssignmentID: assignmentID,
		StudentID:    studentID,
		AttemptID:    attemptID,
	}
	if err := s.repo.UpsertAssignmentSubmission(ctx, submission); err != nil {
		return nil, err
	}

	return dto.ToAssignmentSubmissionResponse(submission), nil
}

func (s *service) UpdateAssignmentSubmissionResult(ctx context.Context, studentID string, assignmentID uint, attemptID uint, score float64) (*dto.AssignmentSubmissionResponse, error) {
	if score < 0 || score > 10 {
		return nil, ErrInvalidSubmissionScore
	}
	score = valuex.RoundFloat(score, 2)

	submission := &model.AssignmentSubmission{
		AssignmentID: assignmentID,
		StudentID:    studentID,
		AttemptID:    attemptID,
		Score:        &score,
	}
	if err := s.repo.UpsertAssignmentSubmissionResult(ctx, submission); err != nil {
		return nil, err
	}

	return dto.ToAssignmentSubmissionResponse(submission), nil
}

func (s *service) GetAssignmentSubmissionSyncStatus(ctx context.Context, studentID string, assignmentID uint, attemptID uint, score float64) (*dto.AssignmentSubmissionSyncStatusResponse, error) {
	submission, err := s.repo.GetAssignmentSubmissionByAssignmentAndStudent(ctx, assignmentID, studentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &dto.AssignmentSubmissionSyncStatusResponse{}, nil
		}
		return nil, err
	}

	attemptMatched := submission.AttemptID == attemptID
	scoreMatched := submission.Score != nil && *submission.Score == valuex.RoundFloat(score, 2)

	return &dto.AssignmentSubmissionSyncStatusResponse{
		Exists:         true,
		AttemptMatched: attemptMatched,
		ScoreMatched:   scoreMatched,
		Synced:         attemptMatched && scoreMatched,
	}, nil
}
