package service

import (
	"context"
	"fmt"
	"math"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/logger"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"

	"gorm.io/datatypes"
)

func createNotificationOutbox(ctx context.Context, repo *repository.Repository, key, recipient, title, body, dataType string, id uint) error {
	return repo.CreateNotificationOutbox(ctx, &model.NotificationOutbox{
		EventKey: key, Recipients: datatypes.JSONSlice[string]{recipient}, Title: title, Body: body,
		Data: datatypes.JSONMap{"id": fmt.Sprint(id), "type": dataType}, NextAttemptAt: time.Now(),
	})
}

func (s *service) ProcessNotificationOutbox(ctx context.Context, limit int) (int, error) {
	if s.notificationsClient == nil {
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
		_, publishErr := s.notificationsClient.CreateNotifications(ctx, &notificationsClient.CreateNotificationsRequest{
			Recipients: []string(event.Recipients), Title: event.Title, Body: event.Body, SourceService: "EXAMS", Data: data, IdempotencyKey: event.EventKey,
		})
		if publishErr != nil {
			attempts := event.Attempts + 1
			delay := time.Duration(math.Min(math.Pow(2, float64(attempts)), 300)) * time.Second
			if markErr := s.repo.MarkNotificationOutboxFailed(ctx, event.ID, attempts, time.Now().Add(delay), publishErr.Error()); markErr != nil {
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
