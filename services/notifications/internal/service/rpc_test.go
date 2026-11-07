package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
	"ego/services/notifications/internal/repository"
)

// fakeRepository giữ idempotency key đã có để giả lập ON CONFLICT DO NOTHING.
type fakeRepository struct {
	repository.Repository
	keys map[string]bool
}

func (f *fakeRepository) Transaction(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

func (f *fakeRepository) CreateNotifications(_ context.Context, items []*model.Notification) ([]*model.Notification, error) {
	var created []*model.Notification
	for _, n := range items {
		if !f.keys[n.IdempotencyKey] {
			f.keys[n.IdempotencyKey] = true
			created = append(created, n)
		}
	}
	return created, nil
}

func TestCreateNotifications(t *testing.T) {
	repo := &fakeRepository{keys: map[string]bool{"assignment:8:user:student-2": true}}
	var broadcasted []string
	svc := New(repo, nil, func(userID string, _ any) { broadcasted = append(broadcasted, userID) })

	created, err := svc.CreateNotifications(context.Background(), dto.CreateNotificationsRequest{
		Recipients: []string{"student-1", " student-1 ", "student-2"}, Title: "New assignment",
		SourceService: "CLASSROOMS", IdempotencyKey: "assignment:8",
	})
	if err != nil {
		t.Fatal(err)
	}
	// student-1 trùng chỉ tạo một lần; student-2 đã có key nên không đếm và không broadcast.
	if created != 1 || !repo.keys["assignment:8:user:student-1"] || strings.Join(broadcasted, ",") != "student-1" {
		t.Fatalf("created=%d keys=%v broadcasted=%v", created, repo.keys, broadcasted)
	}
}

func TestCreateNotificationsInvalid(t *testing.T) {
	many := make([]string, MaxRecipients+1)
	for i := range many {
		many[i] = "u"
	}
	valid := dto.CreateNotificationsRequest{Recipients: []string{"u"}, Title: "T", SourceService: "S", IdempotencyKey: "k"}
	tests := map[string]func(r *dto.CreateNotificationsRequest){
		"no recipients":   func(r *dto.CreateNotificationsRequest) { r.Recipients = nil },
		"too many":        func(r *dto.CreateNotificationsRequest) { r.Recipients = many },
		"blank recipient": func(r *dto.CreateNotificationsRequest) { r.Recipients = []string{" "} },
		"no title":        func(r *dto.CreateNotificationsRequest) { r.Title = " " },
		"no source":       func(r *dto.CreateNotificationsRequest) { r.SourceService = "" },
		"no key":          func(r *dto.CreateNotificationsRequest) { r.IdempotencyKey = "" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			_, err := New(&fakeRepository{keys: map[string]bool{}}, nil, nil).CreateNotifications(context.Background(), req)
			if !errors.Is(err, ErrInvalidNotification) {
				t.Fatalf("err = %v, want ErrInvalidNotification", err)
			}
		})
	}
}
