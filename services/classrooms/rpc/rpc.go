package rpc

import (
	"context"
	"ego/api/gen/go/classrooms"
	"ego/platform/valuex"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	classroomsSocket "ego/services/classrooms/internal/socket"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type server struct {
	classrooms.UnimplementedClassroomServiceServer
	repo      *repository.Repository
	publisher func(userID string, payload any)
}

func New(repo *repository.Repository, publisher func(userID string, payload any)) classrooms.ClassroomServiceServer {
	return &server{
		repo:      repo,
		publisher: publisher,
	}
}

var (
	errAssignmentExamMismatch = errors.New("[ERROR] Assignment does not belong to exam")
	errAssignmentNotOpen      = errors.New("[ERROR] Assignment is not open yet")
	errAssignmentClosed       = errors.New("[ERROR] Assignment is closed")
	errStudentNotApproved     = errors.New("[FORBIDDEN] Student is not approved in classroom")
	errClassroomNotActive     = errors.New("[ERROR] Classroom is not active")
	errAssignmentNotActive    = errors.New("[ERROR] Assignment is not active")
	errInvalidSubmissionScore = errors.New("[ERROR] Invalid submission score")
)

func (s *server) ValidateAssignmentAttempt(ctx context.Context, req *classrooms.ValidateAssignmentAttemptRequest) (*classrooms.ValidateAssignmentAttemptResponse, error) {
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, uint(req.AssignmentId))
	if err != nil {
		return nil, toStatusError(err)
	}

	if assignment.ExamID != uint(req.ExamId) {
		return nil, toStatusError(errAssignmentExamMismatch)
	}
	if !assignment.Classroom.Active {
		return nil, toStatusError(errClassroomNotActive)
	}
	if !assignment.Active {
		return nil, toStatusError(errAssignmentNotActive)
	}

	isApproved, err := s.repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, req.UserId)
	if err != nil {
		return nil, toStatusError(err)
	}
	if !isApproved {
		return nil, toStatusError(errStudentNotApproved)
	}

	now := time.Now()
	if now.Before(assignment.OpensAt) {
		return nil, toStatusError(errAssignmentNotOpen)
	}
	if now.After(assignment.DueAt) {
		return nil, toStatusError(errAssignmentClosed)
	}

	return &classrooms.ValidateAssignmentAttemptResponse{
		AssignmentId:    uint64(assignment.ID),
		ClassroomId:     uint64(assignment.ClassroomID),
		ExamId:          uint64(assignment.ExamID),
		DurationMinutes: int32(assignment.Duration),
		DueAtUnix:       assignment.DueAt.Unix(),
	}, nil
}

func (s *server) CreateAssignmentSubmission(ctx context.Context, req *classrooms.CreateAssignmentSubmissionRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	var submission *model.AssignmentSubmission
	var assignment *model.Assignment
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		var err error
		assignment, err = repo.GetAssignmentForAttemptValidationForUpdate(ctx, uint(req.AssignmentId))
		if err != nil {
			return err
		}
		if !req.AllowInactive && !assignment.Classroom.Active {
			return errClassroomNotActive
		}
		if !req.AllowInactive && !assignment.Active {
			return errAssignmentNotActive
		}
		if !req.AllowInactive {
			isApproved, err := repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, req.StudentId)
			if err != nil {
				return err
			}
			if !isApproved {
				return errStudentNotApproved
			}
		}
		submission = &model.AssignmentSubmission{AssignmentID: assignment.ID, StudentID: req.StudentId, AttemptID: uint(req.AttemptId)}
		return repo.UpsertAssignmentSubmission(ctx, submission)
	})
	if err != nil {
		return nil, toStatusError(err)
	}

	result := &classrooms.AssignmentSubmissionResponse{
		AssignmentId: uint64(submission.AssignmentID),
		StudentId:    submission.StudentID,
		AttemptId:    uint64(submission.AttemptID),
	}
	if submission.Score != nil {
		result.Score = submission.Score
	}
	s.publishGradebookUpdated(assignment, submission, model.Status(submission, time.Now(), assignment.DueAt))
	return result, nil
}

func (s *server) UpdateAssignmentSubmissionResult(ctx context.Context, req *classrooms.UpdateAssignmentSubmissionResultRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	if req.Score < 0 || req.Score > 10 {
		return nil, toStatusError(errInvalidSubmissionScore)
	}
	score := valuex.RoundFloat(req.Score, 2)

	var assignment *model.Assignment
	var submission *model.AssignmentSubmission
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		var err error
		assignment, err = repo.GetAssignmentForAttemptValidationForUpdate(ctx, uint(req.AssignmentId))
		if err != nil {
			return err
		}
		submission = &model.AssignmentSubmission{
			AssignmentID: assignment.ID,
			StudentID:    req.StudentId,
			AttemptID:    uint(req.AttemptId),
			Score:        &score,
		}
		return repo.UpsertAssignmentSubmissionResult(ctx, submission)
	}); err != nil {
		return nil, toStatusError(err)
	}

	result := &classrooms.AssignmentSubmissionResponse{
		AssignmentId: uint64(submission.AssignmentID),
		StudentId:    submission.StudentID,
		AttemptId:    uint64(submission.AttemptID),
	}
	if submission.Score != nil {
		result.Score = submission.Score
	}
	s.publishGradebookUpdated(assignment, submission, model.SubmissionStatusSubmitted)
	return result, nil
}

func (s *server) publishGradebookUpdated(assignment *model.Assignment, submission *model.AssignmentSubmission, status model.SubmissionStatus) {
	if s.publisher == nil || assignment == nil || submission == nil {
		return
	}
	s.publisher(assignment.Classroom.TeacherID, classroomsSocket.GradebookUpdatedEvent{
		Type: classroomsSocket.EventGradebookUpdated,
		Payload: classroomsSocket.GradebookUpdatedPayload{
			ClassroomID:  assignment.ClassroomID,
			AssignmentID: assignment.ID,
			StudentID:    submission.StudentID,
			AttemptID:    submission.AttemptID,
			Status:       string(status),
			Score:        submission.Score,
			UpdatedAt:    time.Now(),
		},
	})
}

func (s *server) GetAssignmentSubmissionSyncStatus(ctx context.Context, req *classrooms.GetAssignmentSubmissionSyncStatusRequest) (*classrooms.GetAssignmentSubmissionSyncStatusResponse, error) {
	submission, err := s.repo.GetAssignmentSubmissionByAssignmentAndStudent(ctx, uint(req.AssignmentId), req.StudentId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &classrooms.GetAssignmentSubmissionSyncStatusResponse{}, nil
		}
		return nil, toStatusError(err)
	}

	attemptMatched := submission.AttemptID == uint(req.AttemptId)
	scoreMatched := submission.Score != nil && *submission.Score == valuex.RoundFloat(req.Score, 2)

	return &classrooms.GetAssignmentSubmissionSyncStatusResponse{
		Exists:         true,
		AttemptMatched: attemptMatched,
		ScoreMatched:   scoreMatched,
		Synced:         attemptMatched && scoreMatched,
	}, nil
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return status.Error(codes.NotFound, "[ERROR] Assignment not found")
	case errors.Is(err, errStudentNotApproved):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, errAssignmentExamMismatch):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, errInvalidSubmissionScore):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, errAssignmentNotOpen),
		errors.Is(err, errAssignmentClosed),
		errors.Is(err, errClassroomNotActive), errors.Is(err, errAssignmentNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
