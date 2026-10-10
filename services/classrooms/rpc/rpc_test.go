package rpc

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func TestToStatusError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code codes.Code
	}{
		{
			name: "not found",
			err:  gorm.ErrRecordNotFound,
			code: codes.NotFound,
		},
		{
			name: "student not approved",
			err:  errStudentNotApproved,
			code: codes.PermissionDenied,
		},
		{
			name: "exam mismatch",
			err:  errAssignmentExamMismatch,
			code: codes.InvalidArgument,
		},
		{
			name: "assignment closed",
			err:  errAssignmentClosed,
			code: codes.FailedPrecondition,
		},
		{
			name: "assignment inactive",
			err:  errAssignmentNotActive,
			code: codes.FailedPrecondition,
		},
		{
			name: "unknown",
			err:  errors.New("boom"),
			code: codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := toStatusError(tt.err)
			if got := status.Code(err); got != tt.code {
				t.Fatalf("expected %s, got %s", tt.code, got)
			}
		})
	}
}
