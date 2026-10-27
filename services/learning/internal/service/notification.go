package service

import (
	"context"
	"fmt"
	"math"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/logger"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

// createNotificationOutbox ghi một thông báo vào outbox trong transaction của repo.
// classroomID = 0 thì không gắn "classroomId"; availableAt zero thì gửi ngay.
func createNotificationOutbox(ctx context.Context, repo repository.Repository, source model.NotificationSource, key string, recipients []string, title, body, dataType string, id, classroomID uint, availableAt time.Time) error {
	if len(recipients) == 0 {
		return nil
	}
	if availableAt.IsZero() {
		availableAt = time.Now()
	}
	data := datatypes.JSONMap{"id": fmt.Sprint(id), "type": dataType}
	if classroomID > 0 {
		data["classroomId"] = fmt.Sprint(classroomID)
	}
	return repo.CreateNotificationOutbox(ctx, &model.NotificationOutbox{
		EventKey: key, Source: source, Recipients: datatypes.JSONSlice[string](recipients), Title: title, Body: body,
		Data: data, NextAttemptAt: availableAt,
	})
}

func (s *service) ProcessNotificationOutbox(ctx context.Context, limit int) (int, error) {
	if s.notificationClient == nil {
		return 0, nil
	}
	events, err := s.repo.ClaimNotificationOutbox(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, event := range events {
		data := make(map[string]string, len(event.Data))
		for key, value := range event.Data {
			data[key] = fmt.Sprint(value)
		}
		_, publishErr := s.notificationClient.CreateNotifications(ctx, &notificationsClient.CreateNotificationsRequest{
			Recipients: []string(event.Recipients), Title: event.Title, Body: event.Body,
			SourceService: string(event.Source), Data: data, IdempotencyKey: event.EventKey,
		})
		if publishErr != nil {
			attempts := event.Attempts + 1
			if markErr := s.repo.MarkNotificationOutboxFailed(ctx, event.ID, attempts, time.Now().Add(notificationRetryDelay(attempts)), publishErr.Error()); markErr != nil {
				return processed, markErr
			}
			logger.Log.Error().Err(publishErr).Uint("event_id", event.ID).Msg("[NOTIFICATION_OUTBOX] Publish failed")
			continue
		}
		if err := s.repo.MarkNotificationOutboxProcessed(ctx, event.ID, time.Now()); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// notificationRetryDelay: 2^attempts giây, tối đa 300 giây.
func notificationRetryDelay(attempts int) time.Duration {
	return time.Duration(math.Min(math.Pow(2, float64(attempts)), 300)) * time.Second
}
