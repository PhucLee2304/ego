package service

import (
	"context"
	"errors"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/push"
	"ego/services/notifications/internal/repository"
)

const MaxRecipients = 500

var ErrInvalidNotification = errors.New("[NOTIFICATIONS] Invalid notification request")

type Service interface {
	// Notification
	GetNotifications(ctx context.Context, userID string, query dto.NotificationsQuery) (*dto.NotificationsResponse, *httpx.HTTPError)
	MarkRead(ctx context.Context, userID string, ids []uint) *httpx.HTTPError
	DeleteNotifications(ctx context.Context, userID string, ids []uint) *httpx.HTTPError

	// Device token
	RegisterDeviceToken(ctx context.Context, userID string, req dto.RegisterDeviceTokenRequest) *httpx.HTTPError
	DeleteDeviceToken(ctx context.Context, userID, token string) *httpx.HTTPError

	// RPC
	CreateNotifications(ctx context.Context, req dto.CreateNotificationsRequest) (int, error)
	DeleteDeviceTokensByUserID(ctx context.Context, userID string) (int, error)

	// Stats (admin)
	GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError)
}

// Broadcaster đẩy event realtime tới mọi kết nối WebSocket của một user.
type Broadcaster func(userID string, payload any)

// NotificationCreatedEvent báo client có thông báo mới để tải lại số chưa đọc.
const NotificationCreatedEvent = "NOTIFICATION_CREATED"

type service struct {
	repo      repository.Repository
	sender    push.Sender
	broadcast Broadcaster
}

func New(repo repository.Repository, sender push.Sender, broadcast Broadcaster) Service {
	return &service{repo: repo, sender: sender, broadcast: broadcast}
}
