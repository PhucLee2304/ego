package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/posts/internal/service"
)

// StartVideoProcessor chuyển mã video đính kèm mỗi interval cho tới khi ctx bị huỷ.
func StartVideoProcessor(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := svc.ProcessVideos(ctx, batchSize); err != nil {
					logger.Log.Error().Err(err).Msg("[WORKER] Video processing failed")
				}
			}
		}
	}()
}
