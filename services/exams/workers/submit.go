package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/exams/internal/service"
)

func StartAutoSubmitExpiredAttempts(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			submitted, err := svc.AutoSubmitExpiredAttempts(ctx, batchSize)
			if err != nil {
				logger.Log.Error().Err(err).Msg("[ATTEMPT] Auto-submit scan failed")
			}
			if submitted > 0 {
				logger.Log.Info().Int("submitted", submitted).Msg("[ATTEMPT] Expired attempts auto-submitted")
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
