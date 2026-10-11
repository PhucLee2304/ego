package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
	"ego/services/notifications/internal/push"
	"ego/services/notifications/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrDeviceTokenConflict = errors.New("[CONFLICT] Device token belongs to another installation")

type Service interface {
	GetNotifications(context.Context, string, dto.NotificationsQuery) (*dto.NotificationsResponse, error)
	RegisterDeviceToken(context.Context, string, dto.RegisterDeviceTokenRequest) error
	DeleteDeviceToken(context.Context, string, string) error
	MarkRead(context.Context, string, []uint) error
	DeleteNotifications(context.Context, string, []uint) error
	CreateNotifications(context.Context, []dto.CreateNotificationRequest) error
}

type service struct {
	repo   *repository.Repository
	sender push.Sender
}

func New(repo *repository.Repository, sender push.Sender) Service {
	return &service{repo: repo, sender: sender}
}

func (s *service) GetNotifications(ctx context.Context, userID string, query dto.NotificationsQuery) (*dto.NotificationsResponse, error) {
	items, total, unread, err := s.repo.GetNotifications(ctx, userID, query.Read, int(query.Limit()), int(query.Offset()))
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.NotificationResponse, len(items))
	for i, item := range items {
		responses[i] = dto.ToNotificationResponse(item)
	}
	return &dto.NotificationsResponse{Notifications: responses, Page: int(query.Page), PageCount: httpx.ToPageCounts(total, query.PageSize), UnreadCount: unread}, nil
}

func (s *service) RegisterDeviceToken(ctx context.Context, userID string, req dto.RegisterDeviceTokenRequest) error {
	err := s.repo.UpsertDeviceToken(ctx, &model.DeviceToken{
		UserID: userID, DeviceID: req.DeviceID, Token: req.Token, Platform: req.Platform,
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDeviceTokenConflict
	}
	return err
}

func (s *service) DeleteDeviceToken(ctx context.Context, userID, token string) error {
	return s.repo.DeleteDeviceToken(ctx, userID, token)
}
func (s *service) MarkRead(ctx context.Context, userID string, ids []uint) error {
	return s.repo.MarkRead(ctx, userID, ids)
}
func (s *service) DeleteNotifications(ctx context.Context, userID string, ids []uint) error {
	return s.repo.DeleteNotifications(ctx, userID, ids)
}

func (s *service) CreateNotifications(ctx context.Context, requests []dto.CreateNotificationRequest) error {
	items := make([]*model.Notification, len(requests))
	for i, req := range requests {
		data := datatypes.JSONMap{}
		for key, value := range req.Data {
			data[key] = value
		}
		items[i] = &model.Notification{UserID: req.UserID, Title: req.Title, Body: req.Body, SourceService: req.SourceService, Data: data, IdempotencyKey: req.IdempotencyKey}
	}
	if err := s.repo.CreateNotifications(ctx, items); err != nil {
		return err
	}
	if s.sender == nil {
		return nil
	}
	for i, req := range requests {
		// OnConflict bỏ qua notification đã tồn tại; chỉ bản ghi vừa insert mới được gửi push.
		if items[i].ID == 0 {
			continue
		}
		pushRequest := req
		pushRequest.Data = make(map[string]string, len(req.Data))
		for key, value := range req.Data {
			pushRequest.Data[key] = value
		}

		go func(req dto.CreateNotificationRequest) {
			// Push là tác vụ best-effort sau khi đã lưu DB, nên không bị hủy khi request HTTP kết thúc.
			pushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()

			tokens, err := s.repo.GetDeviceTokens(pushCtx, req.UserID)
			if err != nil {
				logger.Log.Error().Err(err).Str("user_id", req.UserID).Msg("[NOTIFICATIONS] Failed to load device tokens")
				return
			}
			if len(tokens) == 0 {
				return
			}

			invalid, err := s.sender.Send(pushCtx, tokens, req.Title, req.Body, req.Data)
			if err != nil {
				logger.Log.Error().Err(err).Str("user_id", req.UserID).Msg("[NOTIFICATIONS] FCM send failed")
			}
			if len(invalid) == 0 {
				return
			}

			// Cleanup dùng timeout riêng vì pushCtx có thể đã hết hạn sau khi gọi FCM.
			cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cleanupCancel()
			if err := s.repo.DeleteDeviceTokens(cleanupCtx, invalid); err != nil {
				logger.Log.Error().Err(err).Msg("[NOTIFICATIONS] Failed to delete invalid device tokens")
			}
		}(pushRequest)
	}
	return nil
}
