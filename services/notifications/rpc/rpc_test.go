package rpc

import (
	"context"
	"testing"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/notifications/internal/dto"
)

type fakeCreator struct {
	requests []dto.CreateNotificationRequest
}

func (f *fakeCreator) CreateNotifications(_ context.Context, requests []dto.CreateNotificationRequest) error {
	f.requests = requests
	return nil
}

func TestCreateNotificationsDeduplicatesRecipientsAndBuildsKeys(t *testing.T) {
	creator := &fakeCreator{}
	response, err := New(creator).CreateNotifications(context.Background(), &notificationsClient.CreateNotificationsRequest{
		Recipients: []string{"student-1", "student-1", "student-2"}, Title: "New assignment",
		Body: "Body", SourceService: "CLASSROOMS", Data: map[string]string{"id": "8", "type": "ASSIGNMENT"},
		IdempotencyKey: "assignment:8:created",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.CreatedCount != 2 || len(creator.requests) != 2 {
		t.Fatalf("unexpected recipients: %#v", response)
	}
	if creator.requests[0].IdempotencyKey != "assignment:8:created:user:student-1" || creator.requests[1].IdempotencyKey != "assignment:8:created:user:student-2" {
		t.Fatalf("unexpected idempotency keys: %#v", creator.requests)
	}
}

func TestCreateNotificationsRejectsInvalidBatch(t *testing.T) {
	if _, err := New(&fakeCreator{}).CreateNotifications(context.Background(), &notificationsClient.CreateNotificationsRequest{}); err == nil {
		t.Fatal("expected empty recipients to be rejected")
	}
}
