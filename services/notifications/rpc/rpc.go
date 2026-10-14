package rpc

import (
	"context"
	"fmt"
	"strings"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/notifications/internal/dto"
)

const maxRecipients = 500

type Server struct {
	notificationsClient.UnimplementedNotificationServiceServer
	service notificationCreator
}

type notificationCreator interface {
	CreateNotifications(context.Context, []dto.CreateNotificationRequest) error
}

func New(service notificationCreator) *Server { return &Server{service: service} }

func (s *Server) CreateNotifications(ctx context.Context, req *notificationsClient.CreateNotificationsRequest) (*notificationsClient.CreateNotificationsResponse, error) {
	if len(req.Recipients) == 0 || len(req.Recipients) > maxRecipients {
		return nil, fmt.Errorf("[ERROR] Recipients must contain between 1 and %d users", maxRecipients)
	}
	baseKey := strings.TrimSpace(req.IdempotencyKey)
	if baseKey == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.SourceService) == "" {
		return nil, fmt.Errorf("[ERROR] Title, source service and idempotency key are required")
	}

	seen := make(map[string]struct{}, len(req.Recipients))
	requests := make([]dto.CreateNotificationRequest, 0, len(req.Recipients))
	for _, recipient := range req.Recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient == "" {
			return nil, fmt.Errorf("[ERROR] Recipient is required")
		}
		if _, exists := seen[recipient]; exists {
			continue
		}
		seen[recipient] = struct{}{}
		requests = append(requests, dto.CreateNotificationRequest{
			UserID: recipient, Title: strings.TrimSpace(req.Title), Body: strings.TrimSpace(req.Body),
			SourceService: strings.TrimSpace(req.SourceService), Data: req.Data,
			IdempotencyKey: fmt.Sprintf("%s:user:%s", baseKey, recipient),
		})
	}
	if err := s.service.CreateNotifications(ctx, requests); err != nil {
		return nil, err
	}
	return &notificationsClient.CreateNotificationsResponse{CreatedCount: uint32(len(requests))}, nil
}
