package service

import (
	"context"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/learning/internal/repository"

	"google.golang.org/grpc"
)

type Service interface {
	// Notification outbox
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)
}

// NotificationClient là interface hẹp tới notifications service, để test bằng fake.
type NotificationClient interface {
	CreateNotifications(ctx context.Context, in *notificationsClient.CreateNotificationsRequest, opts ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error)
}

type service struct {
	repo               repository.Repository
	notificationClient NotificationClient
}

func New(repo repository.Repository, notificationClient NotificationClient) Service {
	return &service{repo: repo, notificationClient: notificationClient}
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}
