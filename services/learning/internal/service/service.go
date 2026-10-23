package service

import (
	"context"

	identityClient "ego/api/gen/go/identity"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/learning/internal/media"
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

// IdentityClient là interface hẹp tới identity service: chỉ các method classrooms dùng.
type IdentityClient interface {
	GetUsersInfo(ctx context.Context, in *identityClient.GetUsersInfoRequest, opts ...grpc.CallOption) (*identityClient.GetUsersInfoResponse, error)
	SearchUserIDsByName(ctx context.Context, in *identityClient.SearchUserIDsByNameRequest, opts ...grpc.CallOption) (*identityClient.SearchUserIDsByNameResponse, error)
}

// EventPublisher đẩy payload realtime tới mọi kết nối WebSocket của một user.
type EventPublisher func(userID string, payload any)

type service struct {
	repo               repository.Repository
	notificationClient NotificationClient
	identityClient     IdentityClient
	mediaClient        media.Client
	roomPublisher      EventPublisher // hub /ws/classrooms
	attemptPublisher   EventPublisher // hub /ws/exams
}

func New(
	repo repository.Repository,
	notificationClient NotificationClient,
	identityClient IdentityClient,
	mediaClient media.Client,
	roomPublisher EventPublisher,
	attemptPublisher EventPublisher,
) Service {
	return &service{
		repo:               repo,
		notificationClient: notificationClient,
		identityClient:     identityClient,
		mediaClient:        mediaClient,
		roomPublisher:      roomPublisher,
		attemptPublisher:   attemptPublisher,
	}
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}
