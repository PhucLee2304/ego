package workers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	notifications "ego/api/gen/go/notifications"
	"ego/services/knowledge/internal/model"
	"google.golang.org/grpc"
)

type reminderRepoStub struct {
	cards  []*model.Flashcard
	marked []uint
}

func (f *reminderRepoStub) DueFlashcardsForNotification(_ context.Context, _ time.Time, _ int) ([]*model.Flashcard, error) {
	return f.cards, nil
}

func (f *reminderRepoStub) MarkFlashcardNotified(_ context.Context, id uint, _ time.Time) error {
	f.marked = append(f.marked, id)
	return nil
}

type notificationClientStub struct {
	notifications.NotificationServiceClient
	request *notifications.CreateNotificationsRequest
	err     error
}

func (f *notificationClientStub) CreateNotifications(_ context.Context, req *notifications.CreateNotificationsRequest, _ ...grpc.CallOption) (*notifications.CreateNotificationsResponse, error) {
	f.request = req
	return &notifications.CreateNotificationsResponse{CreatedCount: 1}, f.err
}

func TestProcessFlashcardDueSendsSpecificCardAndMarksOnlyAfterSuccess(t *testing.T) {
	due := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	card := &model.Flashcard{UserID: "learner", DeckID: 7, WordID: 8, NextReviewAt: due, Word: model.Word{Word: "adjective"}}
	card.ID = 42
	repo := &reminderRepoStub{cards: []*model.Flashcard{card}}
	client := &notificationClientStub{}

	count, err := ProcessFlashcardDue(context.Background(), repo, client, due, 10)
	if err != nil || count != 1 || len(repo.marked) != 1 || repo.marked[0] != 42 {
		t.Fatalf("reminder was not dispatched and marked: count=%d err=%v marked=%v", count, err, repo.marked)
	}
	if client.request.GetRecipients()[0] != "learner" || client.request.GetData()["type"] != "FLASHCARD_DUE" || client.request.GetData()["deckId"] != "7" || !strings.Contains(client.request.GetBody(), "adjective") || !strings.Contains(client.request.GetIdempotencyKey(), "42") {
		t.Fatalf("wrong reminder context: %#v", client.request)
	}

	repo.marked = nil
	client.err = errors.New("notifications unavailable")
	count, err = ProcessFlashcardDue(context.Background(), repo, client, due, 10)
	if err == nil || count != 0 || len(repo.marked) != 0 {
		t.Fatalf("failed delivery must remain retryable: count=%d err=%v marked=%v", count, err, repo.marked)
	}
}
