package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ego/services/learning/internal/dto"
)

func TestCreateAssignmentsRejectsDurationExceedingOpenWindow(t *testing.T) {
	opensAt := time.Date(2026, time.October, 5, 8, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		duration int
		dueAt    time.Time
	}{
		{
			name:     "duration exceeds open window",
			duration: 61,
			dueAt:    opensAt.Add(time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &service{}
			_, err := s.CreateAssignments(context.Background(), "teacher-id", dto.CreateAssignmentsRequest{
				Title: "Assignment",
				Schedules: []dto.AssignmentScheduleRequest{{
					ClassroomID: 1,
					Duration:    tt.duration,
					OpensAt:     opensAt,
					DueAt:       tt.dueAt,
				}},
			})

			if !errors.Is(err, ErrAssignmentDuration) {
				t.Fatalf("expected ErrAssignmentDuration, got %v", err)
			}
		})
	}
}
