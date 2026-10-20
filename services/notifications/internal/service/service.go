package service

import (
	"context"
	"errors"
	"net/http"
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

type Service interface {
	GetNotifications(context.Context, string, dto.NotificationsQuery) (*dto.NotificationsResponse, *httpx.HTTPError)
	RegisterDeviceToken(context.Context, string, dto.RegisterDeviceTokenRequest) *httpx.HTTPError
	DeleteDeviceToken(context.Context, string, string) *httpx.HTTPError
	MarkRead(context.Context, string, []uint) *httpx.HTTPError
	DeleteNotifications(context.Context, string, []uint) *httpx.HTTPError
	CreateNotifications(context.Context, []dto.CreateNotificationRequest) *httpx.HTTPError
}

type service struct {
	repo   *repository.Repository
	sender push.Sender
}

func New(repo *repository.Repository, sender push.Sender) Service {
	return &service{repo: repo, sender: sender}
}

func (s *service) GetNotifications(ctx context.Context, userID string, query dto.NotificationsQuery) (*dto.NotificationsResponse, *httpx.HTTPError) {
	items, total, unread, err := s.repo.GetNotifications(ctx, userID, query.Read, int(query.Limit()), int(query.Offset()))
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get notifications", err)
	}
	responses := make([]*dto.NotificationResponse, len(items))
	for i, item := range items {
		responses[i] = dto.ToNotificationResponse(item)
	}
	return &dto.NotificationsResponse{Notifications: responses, Page: int(query.Page), PageCount: httpx.ToPageCounts(total, query.PageSize), UnreadCount: unread}, nil
}

func (s *service) RegisterDeviceToken(ctx context.Context, userID string, req dto.RegisterDeviceTokenRequest) *httpx.HTTPError {
	err := s.repo.UpsertDeviceToken(ctx, &model.DeviceToken{
		UserID: userID, DeviceID: req.DeviceID, Token: req.Token, Platform: req.Platform,
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return httpx.NewError(http.StatusConflict, "[CONFLICT] Device token belongs to another installation", nil)
	}
	if err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to register device token", err)
	}
	return nil
}

func (s *service) DeleteDeviceToken(ctx context.Context, userID, token string) *httpx.HTTPError {
	if err := s.repo.DeleteDeviceToken(ctx, userID, token); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete device token", err)
	}
	return nil
}
func (s *service) MarkRead(ctx context.Context, userID string, ids []uint) *httpx.HTTPError {
	if err := s.repo.MarkRead(ctx, userID, ids); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to mark notifications as read", err)
	}
	return nil
}
func (s *service) DeleteNotifications(ctx context.Context, userID string, ids []uint) *httpx.HTTPError {
	if err := s.repo.DeleteNotifications(ctx, userID, ids); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete notifications", err)
	}
	return nil
}

func (s *service) CreateNotifications(ctx context.Context, requests []dto.CreateNotificationRequest) *httpx.HTTPError {
	items := make([]*model.Notification, len(requests))
	for i, req := range requests {
		data := datatypes.JSONMap{}
		for key, value := range req.Data {
			data[key] = value
		}
		items[i] = &model.Notification{UserID: req.UserID, Title: req.Title, Body: req.Body, SourceService: req.SourceService, Data: data, IdempotencyKey: req.IdempotencyKey}
	}
	if err := s.repo.CreateNotifications(ctx, items); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to create notifications", err)
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
