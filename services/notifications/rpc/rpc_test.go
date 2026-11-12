package rpc

import (
	"context"
	"testing"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeService struct {
	service.Service
	req dto.CreateNotificationsRequest
	err error
}

func (f *fakeService) CreateNotifications(_ context.Context, req dto.CreateNotificationsRequest) (int, error) {
	f.req = req
	return 2, f.err
}

func TestCreateNotificationsMapsRequest(t *testing.T) {
	svc := &fakeService{}
	resp, err := New(svc, nil).CreateNotifications(context.Background(), &notificationsClient.CreateNotificationsRequest{
		Recipients: []string{"a", "b"}, Title: "T", SourceService: "POSTS", IdempotencyKey: "k",
	})
	if err != nil || resp.CreatedCount != 2 || svc.req.IdempotencyKey != "k" || len(svc.req.Recipients) != 2 {
		t.Fatalf("resp=%v err=%v req=%+v", resp, err, svc.req)
	}
}

func TestCreateNotificationsInvalidArgument(t *testing.T) {
	_, err := New(&fakeService{err: service.ErrInvalidNotification}, nil).CreateNotifications(context.Background(), &notificationsClient.CreateNotificationsRequest{})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, want InvalidArgument", status.Code(err))
	}
}
