package service

import (
	"context"
	"net/http"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
)

// **Reviewed**
func (s *service) GetNotifications(ctx context.Context, userID string, query dto.NotificationsQuery) (*dto.NotificationsResponse, *httpx.HTTPError) {
	var types []string
	if query.Category != "" {
		types = model.CategoryTypes[query.Category]
	}
	items, total, unread, err := s.repo.GetNotifications(ctx, userID, query.Read, types, int(query.Limit()), int(query.Offset()))
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get notifications", err)
	}
	byType, err := s.repo.CountUnreadByType(ctx, userID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get notifications", err)
	}
	byCategory := map[string]int64{}
	for dataType, count := range byType {
		if category := model.CategoryOf(dataType); category != "" {
			byCategory[category] += count
		}
	}
	responses := make([]*dto.NotificationResponse, len(items))
	for i, item := range items {
		responses[i] = dto.ToNotificationResponse(item)
	}
	return &dto.NotificationsResponse{
		Notifications:    responses,
		Page:             int(query.Page),
		PageCount:        httpx.ToPageCounts(total, query.PageSize),
		UnreadCount:      unread,
		UnreadByCategory: byCategory,
	}, nil
}

// **Reviewed**
func (s *service) MarkRead(ctx context.Context, userID string, ids []uint) *httpx.HTTPError {
	if err := s.repo.MarkRead(ctx, userID, ids); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to mark notifications as read", err)
	}
	return nil
}

// **Reviewed**
func (s *service) DeleteNotifications(ctx context.Context, userID string, ids []uint) *httpx.HTTPError {
	if err := s.repo.DeleteNotifications(ctx, userID, ids); err != nil {
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete notifications", err)
	}
	return nil
}
