package rpc

import (
	"context"
	"ego/api/gen/go/classrooms"
	"ego/platform/valuex"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type server struct {
	classrooms.UnimplementedClassroomServiceServer
	repo *repository.Repository
}

func New(repo *repository.Repository) classrooms.ClassroomServiceServer {
	return &server{
		repo: repo,
	}
}

var (
	errAssignmentExamMismatch = errors.New("[ERROR] Assignment does not belong to exam")
	errAssignmentNotOpen      = errors.New("[ERROR] Assignment is not open yet")
	errAssignmentClosed       = errors.New("[ERROR] Assignment is closed")
	errStudentNotApproved     = errors.New("[FORBIDDEN] Student is not approved in classroom")
	errClassroomNotActive     = errors.New("[ERROR] Classroom is not active")
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
		AssignmentId: uint64(assignment.ID),
		ClassroomId:  uint64(assignment.ClassroomID),
		ExamId:       uint64(assignment.ExamID),
	}, nil
}

func (s *server) CreateAssignmentSubmission(ctx context.Context, req *classrooms.CreateAssignmentSubmissionRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	assignment, err := s.repo.GetAssignmentForAttemptValidation(ctx, uint(req.AssignmentId))
	if err != nil {
		return nil, toStatusError(err)
	}
	if !assignment.Classroom.Active {
		return nil, toStatusError(errClassroomNotActive)
	}

	isApproved, err := s.repo.IsStudentApprovedInClassroom(ctx, assignment.ClassroomID, req.StudentId)
	if err != nil {
		return nil, toStatusError(err)
	}
	if !isApproved {
		return nil, toStatusError(errStudentNotApproved)
	}

	submission := &model.AssignmentSubmission{
		AssignmentID: uint(req.AssignmentId),
		StudentID:    req.StudentId,
		AttemptID:    uint(req.AttemptId),
	}
	if err := s.repo.UpsertAssignmentSubmission(ctx, submission); err != nil {
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
	return result, nil
}

func (s *server) UpdateAssignmentSubmissionResult(ctx context.Context, req *classrooms.UpdateAssignmentSubmissionResultRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	if req.Score < 0 || req.Score > 10 {
		return nil, toStatusError(errInvalidSubmissionScore)
	}
	score := valuex.RoundFloat(req.Score, 2)

	submission := &model.AssignmentSubmission{
		AssignmentID: uint(req.AssignmentId),
		StudentID:    req.StudentId,
		AttemptID:    uint(req.AttemptId),
		Score:        &score,
	}
	if err := s.repo.UpsertAssignmentSubmissionResult(ctx, submission); err != nil {
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
	return result, nil
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
		errors.Is(err, errClassroomNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
