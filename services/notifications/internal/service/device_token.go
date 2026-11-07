package service

import (
	"context"
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
	"ego/services/notifications/internal/repository"

	"gorm.io/gorm"
)

// **Reviewed**
func (s *service) RegisterDeviceToken(ctx context.Context, userID string, req dto.RegisterDeviceTokenRequest) *httpx.HTTPError {
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		return repo.UpsertDeviceToken(ctx, &model.DeviceToken{
			UserID:   userID,
			DeviceID: req.DeviceID,
			Token:    req.Token,
			Platform: req.Platform,
		})
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return httpx.NewError(http.StatusConflict, "[CONFLICT] Device token is being registered, please retry", err)
	}
	if err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to register device token", err)
	}
	return nil
}

// **Reviewed**
func (s *service) DeleteDeviceToken(ctx context.Context, userID, token string) *httpx.HTTPError {
	if err := s.repo.DeleteDeviceToken(ctx, userID, token); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete device token", err)
	}
	return nil
}
