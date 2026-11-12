package service

import (
	"context"
	"fmt"
	"math"
	"time"

	notifications "ego/api/gen/go/notifications"
)

func (s *service) ProcessNotificationOutbox(ctx context.Context, limit int) (int, error) {
	if s.notificationClient == nil {
		return 0, nil
	}
	items, err := s.repo.ClaimOutbox(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, v := range items {
		data := map[string]string{}
		for k, x := range v.Data {
			data[k] = fmt.Sprint(x)
		}
		_, e := s.notificationClient.CreateNotifications(ctx, &notifications.CreateNotificationsRequest{Recipients: []string(v.Recipients), Title: v.Title, Body: v.Body, SourceService: "POSTS", Data: data, IdempotencyKey: v.EventKey})
		if e != nil {
			attempts := v.Attempts + 1
			delay := time.Duration(math.Min(math.Pow(2, float64(attempts)), 300)) * time.Second
			if x := s.repo.MarkOutboxFailed(ctx, v.ID, attempts, time.Now().Add(delay), e.Error()); x != nil {
				return done, x
			}
			continue
		}
		if e = s.repo.MarkOutboxProcessed(ctx, v.ID, time.Now()); e != nil {
			return done, e
		}
		done++
	}
	return done, nil
}
