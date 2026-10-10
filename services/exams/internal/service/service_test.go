package service

import (
	"context"
	"testing"
	"time"

	"ego/services/exams/internal/dto"
)

func TestCreatePrivateExamRejectsEmptyQuestions(t *testing.T) {
	svc := &service{}
	if _, err := svc.CreatePrivateExam(context.Background(), dto.CreatePrivateExamRequest{Title: "Empty exam"}); err == nil {
		t.Fatal("expected an exam without questions to be rejected")
	}
}

func TestAssignmentAttemptExpiresAt(t *testing.T) {
	startedAt := time.Date(2026, time.October, 5, 8, 0, 0, 0, time.UTC)

	t.Run("uses assignment duration", func(t *testing.T) {
		dueAt := startedAt.Add(2 * time.Hour)
		got := assignmentAttemptExpiresAt(startedAt, 45, dueAt)
		want := startedAt.Add(45 * time.Minute)
		if !got.Equal(want) {
			t.Fatalf("expected %s, got %s", want, got)
		}
	})

	t.Run("does not exceed assignment deadline", func(t *testing.T) {
		dueAt := startedAt.Add(20 * time.Minute)
		got := assignmentAttemptExpiresAt(startedAt, 45, dueAt)
		if !got.Equal(dueAt) {
			t.Fatalf("expected %s, got %s", dueAt, got)
		}
	})
}
