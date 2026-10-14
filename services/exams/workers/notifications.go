package workers

import (
	"context"
	"ego/platform/logger"
	"ego/services/exams/internal/service"
	"time"
)

func StartNotificationOutbox(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := svc.ProcessNotificationOutbox(ctx, batchSize); err != nil {
					logger.Log.Error().Err(err).Msg("[NOTIFICATION_OUTBOX] Processing failed")
				}
			}
		}
	}()
}
