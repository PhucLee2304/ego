package service

import (
	"testing"
	"time"
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
