package rpc

import (
	"context"
	"ego/api/gen/go/classrooms"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/service"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type server struct {
	classrooms.UnimplementedClassroomServiceServer
	service service.Service
}

func New(service service.Service) classrooms.ClassroomServiceServer {
	return &server{
		service: service,
	}
}

func (s *server) ValidateAssignmentAttempt(ctx context.Context, req *classrooms.ValidateAssignmentAttemptRequest) (*classrooms.ValidateAssignmentAttemptResponse, error) {
	resp, err := s.service.ValidateAssignmentAttempt(ctx, req.UserId, uint(req.AssignmentId), uint(req.ExamId))
	if err != nil {
		return nil, toStatusError(err)
	}

	return &classrooms.ValidateAssignmentAttemptResponse{
		AssignmentId: uint64(resp.AssignmentID),
		ClassroomId:  uint64(resp.ClassroomID),
		ExamId:       uint64(resp.ExamID),
	}, nil
}

func (s *server) CreateAssignmentSubmission(ctx context.Context, req *classrooms.CreateAssignmentSubmissionRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	resp, err := s.service.CreateAssignmentSubmission(ctx, req.StudentId, uint(req.AssignmentId), uint(req.AttemptId))
	if err != nil {
		return nil, toStatusError(err)
	}

	return toAssignmentSubmissionResponse(resp), nil
}

func (s *server) UpdateAssignmentSubmissionResult(ctx context.Context, req *classrooms.UpdateAssignmentSubmissionResultRequest) (*classrooms.AssignmentSubmissionResponse, error) {
	resp, err := s.service.UpdateAssignmentSubmissionResult(ctx, req.StudentId, uint(req.AssignmentId), uint(req.AttemptId), req.Score)
	if err != nil {
		return nil, toStatusError(err)
	}

	return toAssignmentSubmissionResponse(resp), nil
}

func (s *server) GetAssignmentSubmissionSyncStatus(ctx context.Context, req *classrooms.GetAssignmentSubmissionSyncStatusRequest) (*classrooms.GetAssignmentSubmissionSyncStatusResponse, error) {
	resp, err := s.service.GetAssignmentSubmissionSyncStatus(ctx, req.StudentId, uint(req.AssignmentId), uint(req.AttemptId), req.Score)
	if err != nil {
		return nil, toStatusError(err)
	}

	return &classrooms.GetAssignmentSubmissionSyncStatusResponse{
		Exists:         resp.Exists,
		AttemptMatched: resp.AttemptMatched,
		ScoreMatched:   resp.ScoreMatched,
		Synced:         resp.Synced,
	}, nil
}

func toAssignmentSubmissionResponse(resp *dto.AssignmentSubmissionResponse) *classrooms.AssignmentSubmissionResponse {
	result := &classrooms.AssignmentSubmissionResponse{
		AssignmentId: uint64(resp.AssignmentID),
		StudentId:    resp.StudentID,
		AttemptId:    uint64(resp.AttemptID),
	}
	if resp.Score != nil {
		result.Score = resp.Score
	}
	return result
}

func toStatusError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return status.Error(codes.NotFound, "[ERROR] Assignment not found")
	case errors.Is(err, service.ErrStudentNotApproved):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, service.ErrAssignmentExamMismatch):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrInvalidSubmissionScore):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrAssignmentNotOpen),
		errors.Is(err, service.ErrAssignmentClosed),
		errors.Is(err, service.ErrClassroomNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
