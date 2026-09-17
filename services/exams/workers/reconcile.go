package workers

import (
	"context"
	"time"

	"ego/platform/logger"
	"ego/services/exams/internal/service"
)

func StartClassroomAssignmentSubmissionReconciler(ctx context.Context, svc service.Service, interval time.Duration, batchSize int) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			enqueued, err := svc.ReconcileClassroomAssignmentSubmissions(ctx, batchSize)
			if err != nil {
				logger.Log.Error().Err(err).Msg("[OUTBOX] Reconcile scan failed")
			}
			if enqueued > 0 {
				logger.Log.Warn().Int("enqueued", enqueued).Msg("[OUTBOX] Reconcile enqueued missing classroom submission sync events")
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
