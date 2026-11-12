package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/identity/internal/service"
)

// StartUserBlockedOutbox chạy vòng lặp xử lý outbox "user bị khoá" mỗi interval cho tới khi ctx bị huỷ.
func StartUserBlockedOutbox(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := svc.ProcessUserBlockedOutbox(ctx, batchSize); err != nil {
					logger.Log.Error().Err(err).Msg("[WORKER] User blocked outbox processing failed")
				}
			}
		}
	}()
}
