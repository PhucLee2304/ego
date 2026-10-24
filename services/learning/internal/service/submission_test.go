package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/gorm"
)

func TestAssignmentAttemptExpiresAt(t *testing.T) {
	startedAt := time.Date(2026, time.October, 5, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		dueAt time.Time
		want  time.Time
	}{
		{"uses assignment duration", startedAt.Add(2 * time.Hour), startedAt.Add(45 * time.Minute)},
		{"does not exceed assignment deadline", startedAt.Add(20 * time.Minute), startedAt.Add(20 * time.Minute)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := assignmentAttemptExpiresAt(startedAt, 45, tt.dueAt); !got.Equal(tt.want) {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

type fakeAssignmentRepository struct {
	repository.Repository
	assignment *model.Assignment
	err        error
	approved   bool
}

func (f *fakeAssignmentRepository) GetAssignmentForAttemptValidation(context.Context, uint) (*model.Assignment, error) {
	return f.assignment, f.err
}

func (f *fakeAssignmentRepository) IsStudentApprovedInClassroom(context.Context, uint, string) (bool, error) {
	return f.approved, nil
}

// Thay TestToStatusError của classrooms/rpc: lỗi kiểm tra assignment phải ra đúng
// HTTP status + message mà exams trước đây nhận qua gRPC.
func TestValidateAssignmentAttemptErrors(t *testing.T) {
	open := func() *model.Assignment {
		return &model.Assignment{ExamID: 7, Active: true, Classroom: model.Classroom{Active: true},
			OpensAt: time.Now().Add(-time.Hour), DueAt: time.Now().Add(time.Hour)}
	}
	tests := []struct {
		name    string
		repo    *fakeAssignmentRepository
		status  int
		message string
	}{
		{"not found", &fakeAssignmentRepository{err: gorm.ErrRecordNotFound}, http.StatusNotFound, "[ERROR] Assignment not found"},
		{"exam mismatch", &fakeAssignmentRepository{assignment: func() *model.Assignment { a := open(); a.ExamID = 8; return a }(), approved: true}, http.StatusBadRequest, "[ERROR] Assignment does not belong to exam"},
		{"classroom inactive", &fakeAssignmentRepository{assignment: func() *model.Assignment { a := open(); a.Classroom.Active = false; return a }(), approved: true}, http.StatusBadRequest, "[ERROR] Classroom is not active"},
		{"assignment inactive", &fakeAssignmentRepository{assignment: func() *model.Assignment { a := open(); a.Active = false; return a }(), approved: true}, http.StatusBadRequest, "[ERROR] Assignment is not active"},
		{"student not approved", &fakeAssignmentRepository{assignment: open()}, http.StatusForbidden, "[FORBIDDEN] Student is not approved in classroom"},
		{"not open", &fakeAssignmentRepository{assignment: func() *model.Assignment { a := open(); a.OpensAt = time.Now().Add(time.Minute); return a }(), approved: true}, http.StatusBadRequest, "[ERROR] Assignment is not open yet"},
		{"closed", &fakeAssignmentRepository{assignment: func() *model.Assignment { a := open(); a.DueAt = time.Now().Add(-time.Minute); return a }(), approved: true}, http.StatusBadRequest, "[ERROR] Assignment is closed"},
		{"unknown", &fakeAssignmentRepository{err: errors.New("boom")}, http.StatusInternalServerError, "[ERROR] Failed to create attempt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&service{}).validateAssignmentAttempt(context.Background(), tt.repo, "student", 1, 7)
			httpErr := httpx.FromError(err, "[ERROR] Failed to create attempt")
			if httpErr == nil || httpErr.StatusCode != tt.status || httpErr.Message != tt.message {
				t.Fatalf("got %v, want %d %q", httpErr, tt.status, tt.message)
			}
		})
	}

	if assignment, err := (&service{}).validateAssignmentAttempt(context.Background(), &fakeAssignmentRepository{assignment: open(), approved: true}, "student", 1, 7); err != nil || assignment == nil {
		t.Fatalf("open assignment must pass: %v", err)
	}
}
