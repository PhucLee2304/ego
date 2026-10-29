package service

import (
	"context"
	"errors"
	"testing"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"google.golang.org/grpc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type fakeRepository struct {
	repository.Repository
	created   []*model.NotificationOutbox
	claimed   []*model.NotificationOutbox
	processed []uint
	failed    map[uint]int
	nextAt    map[uint]time.Time
}

func (f *fakeRepository) CreateNotificationOutbox(_ context.Context, event *model.NotificationOutbox) error {
	f.created = append(f.created, event)
	return nil
}

func (f *fakeRepository) ClaimNotificationOutbox(context.Context, time.Time, int) ([]*model.NotificationOutbox, error) {
	return f.claimed, nil
}

func (f *fakeRepository) MarkNotificationOutboxProcessed(_ context.Context, id uint, _ time.Time) error {
	f.processed = append(f.processed, id)
	return nil
}

func (f *fakeRepository) MarkNotificationOutboxFailed(_ context.Context, id uint, attempts int, next time.Time, _ string) error {
	if f.failed == nil {
		f.failed, f.nextAt = map[uint]int{}, map[uint]time.Time{}
	}
	f.failed[id] = attempts
	f.nextAt[id] = next
	return nil
}

type fakeNotificationClient struct {
	requests []*notificationsClient.CreateNotificationsRequest
	failKeys map[string]bool
}

func (f *fakeNotificationClient) CreateNotifications(_ context.Context, in *notificationsClient.CreateNotificationsRequest, _ ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error) {
	f.requests = append(f.requests, in)
	if f.failKeys[in.IdempotencyKey] {
		return nil, errors.New("unavailable")
	}
	return &notificationsClient.CreateNotificationsResponse{}, nil
}

func outboxEvent(id uint, key string, source model.NotificationSource, attempts int) *model.NotificationOutbox {
	return &model.NotificationOutbox{
		Model: gorm.Model{ID: id}, EventKey: key, Source: source, Attempts: attempts,
		Recipients: datatypes.JSONSlice[string]{"u1"}, Title: "t", Body: "b",
		Data: datatypes.JSONMap{"id": "7", "type": "ATTEMPT"},
	}
}

func TestProcessNotificationOutbox(t *testing.T) {
	repo := &fakeRepository{claimed: []*model.NotificationOutbox{
		outboxEvent(1, "attempt:7:submitted", model.NotificationSourceAttempt, 0),
		outboxEvent(2, "classroom:3:member:9:approved", model.NotificationSourceMember, 0),
		outboxEvent(3, "room:5:started", model.NotificationSourceRoom, 2),
	}}
	client := &fakeNotificationClient{failKeys: map[string]bool{"room:5:started": true}}
	svc := New(repo, nil, client, nil, nil, nil, nil, nil)

	before := time.Now()
	processed, err := svc.ProcessNotificationOutbox(context.Background(), 10)
	if err != nil {
		t.Fatalf("ProcessNotificationOutbox() error = %v", err)
	}
	if processed != 2 {
		t.Fatalf("processed = %d, want 2", processed)
	}

	t.Run("gửi đúng source và idempotency key", func(t *testing.T) {
		want := map[string]string{
			"attempt:7:submitted":           "ATTEMPT",
			"classroom:3:member:9:approved": "MEMBER",
			"room:5:started":                "ROOM",
		}
		for _, req := range client.requests {
			if req.SourceService != want[req.IdempotencyKey] {
				t.Errorf("%s: SourceService = %q, want %q", req.IdempotencyKey, req.SourceService, want[req.IdempotencyKey])
			}
			if req.Data["id"] != "7" || req.Data["type"] != "ATTEMPT" {
				t.Errorf("%s: Data = %v", req.IdempotencyKey, req.Data)
			}
		}
	})

	t.Run("đánh dấu processed và failed", func(t *testing.T) {
		if len(repo.processed) != 2 || repo.processed[0] != 1 || repo.processed[1] != 2 {
			t.Errorf("processed ids = %v, want [1 2]", repo.processed)
		}
		if repo.failed[3] != 3 {
			t.Errorf("failed attempts = %d, want 3", repo.failed[3])
		}
		if next := repo.nextAt[3]; next.Before(before.Add(8*time.Second)) || next.After(time.Now().Add(8*time.Second)) {
			t.Errorf("next attempt = %s, want ~now+8s", next)
		}
	})
}

func TestProcessNotificationOutboxWithoutClient(t *testing.T) {
	repo := &fakeRepository{claimed: []*model.NotificationOutbox{outboxEvent(1, "k", model.NotificationSourceAttempt, 0)}}
	processed, err := New(repo, nil, nil, nil, nil, nil, nil, nil).ProcessNotificationOutbox(context.Background(), 10)
	if err != nil || processed != 0 || len(repo.processed) != 0 {
		t.Fatalf("processed = %d, err = %v, marked = %v; want 0, nil, none", processed, err, repo.processed)
	}
}

func TestNotificationRetryDelay(t *testing.T) {
	tests := []struct {
		attempts int
		want     time.Duration
	}{
		{1, 2 * time.Second},
		{3, 8 * time.Second},
		{8, 256 * time.Second},
		{9, 300 * time.Second},
		{20, 300 * time.Second},
	}
	for _, tt := range tests {
		if got := notificationRetryDelay(tt.attempts); got != tt.want {
			t.Errorf("notificationRetryDelay(%d) = %s, want %s", tt.attempts, got, tt.want)
		}
	}
}

func TestCreateNotificationOutbox(t *testing.T) {
	sendAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name          string
		recipients    []string
		classroomID   uint
		availableAt   time.Time
		wantCreated   bool
		wantClassroom bool
	}{
		{name: "không có người nhận thì bỏ qua", recipients: nil},
		{name: "không có classroomID thì không gắn classroomId", recipients: []string{"u1"}, wantCreated: true},
		{name: "có classroomID thì gắn classroomId và giờ gửi", recipients: []string{"u1", "u2"}, classroomID: 4, availableAt: sendAt, wantCreated: true, wantClassroom: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{}
			if err := createNotificationOutbox(context.Background(), repo, model.NotificationSourceAssignment, "key", tt.recipients, "t", "b", "ASSIGNMENT", 9, tt.classroomID, tt.availableAt); err != nil {
				t.Fatalf("error = %v", err)
			}
			if !tt.wantCreated {
				if len(repo.created) != 0 {
					t.Fatalf("created %d events, want 0", len(repo.created))
				}
				return
			}
			event := repo.created[0]
			if event.Source != model.NotificationSourceAssignment || event.Data["id"] != "9" || event.Data["type"] != "ASSIGNMENT" {
				t.Errorf("event = %+v", event)
			}
			if _, ok := event.Data["classroomId"]; ok != tt.wantClassroom {
				t.Errorf("has classroomId = %v, want %v", ok, tt.wantClassroom)
			}
			if !tt.availableAt.IsZero() && !event.NextAttemptAt.Equal(tt.availableAt) {
				t.Errorf("NextAttemptAt = %s, want %s", event.NextAttemptAt, tt.availableAt)
			}
			if tt.availableAt.IsZero() && event.NextAttemptAt.IsZero() {
				t.Error("NextAttemptAt is zero, want now")
			}
		})
	}
}
