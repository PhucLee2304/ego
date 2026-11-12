package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/posts/internal/service"
)

func StartUploadCleanup(ctx context.Context, svc service.Service, interval time.Duration, limit int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := svc.CleanupExpiredUploads(ctx, limit); err != nil {
					logger.Log.Error().Err(err).Msg("[WORKER] Upload cleanup failed")
				}
			}
		}
	}()
}
