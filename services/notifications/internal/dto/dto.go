package dto

import (
	"time"

	"ego/platform/httpx"
	"ego/services/notifications/internal/model"
)

type NotificationsQuery struct {
	httpx.PaginationQuery
	Read *bool `schema:"read"`
}

type NotificationResponse struct {
	ID        uint              `json:"id"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data"`
	ReadAt    *time.Time        `json:"readAt,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

type NotificationsResponse struct {
	Notifications []*NotificationResponse `json:"notifications"`
	Page          int                     `json:"page"`
	PageCount     int                     `json:"pageCount"`
	UnreadCount   int64                   `json:"unreadCount"`
}

type RegisterDeviceTokenRequest struct {
	Token    string              `json:"token" validate:"required,notblank"`
	Platform model.TokenPlatform `json:"platform" validate:"required,oneof=WEB ANDROID IOS"`
}

type DeleteDeviceTokenRequest struct {
	Token string `json:"token" validate:"required,notblank"`
}

type NotificationIDsRequest struct {
	IDs []uint `json:"ids" validate:"omitempty,dive,gt=0"`
}

type DeleteNotificationsRequest struct {
	IDs []uint `json:"ids" validate:"required,min=1,dive,gt=0"`
}

type CreateNotificationRequest struct {
	UserID         string
	Title          string
	Body           string
	SourceService  string
	IdempotencyKey string
	Data           map[string]string
}

func ToNotificationResponse(notification *model.Notification) *NotificationResponse {
	data := make(map[string]string, len(notification.Data))
	for key, value := range notification.Data {
		if text, ok := value.(string); ok {
			data[key] = text
		}
	}
	return &NotificationResponse{
		ID: notification.ID, Title: notification.Title, Body: notification.Body,
		Data: data, ReadAt: notification.ReadAt, CreatedAt: notification.CreatedAt,
	}
}
