package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/posts/internal/service"
)

func StartNotificationOutbox(ctx context.Context, postService *service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := postService.ProcessNotificationOutbox(ctx, batchSize); err != nil {
					logger.Log.Error().Err(err).Msg("[POSTS] Notification outbox failed")
				}
			}
		}
	}()
}
