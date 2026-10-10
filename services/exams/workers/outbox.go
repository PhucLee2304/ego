package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/exams/internal/service"
)

func StartOutboxPublisher(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			processed, err := svc.ProcessOutboxEvents(ctx, batchSize)
			if err != nil {
				logger.Log.Error().Err(err).Msg("[OUTBOX] Publish scan failed")
			}
			if processed > 0 {
				logger.Log.Info().Int("processed", processed).Msg("[OUTBOX] Events published")
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
