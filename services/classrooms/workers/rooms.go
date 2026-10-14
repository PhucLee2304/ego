package workers

import (
	"context"
	"ego/platform/logger"
	"ego/services/classrooms/internal/service"
	"time"
)

func StartRoomLifecycle(ctx context.Context, svc service.Service, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := svc.ProcessRoomLifecycle(ctx); err != nil {
					logger.Log.Error().Err(err).Msg("[ROOM] Lifecycle processing failed")
				}
			}
		}
	}()
}
