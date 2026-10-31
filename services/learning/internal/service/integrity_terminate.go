package service

import (
	"context"

	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"
)

// terminateAttempt đình chỉ attempt đang làm trong tx của repo (mục 1.2): chấm bài
// như nộp thường (RawScore), ép Score = 0, chốt qua finalizeAttempt, cập nhật
// submission và outbox. Push ATTEMPT_TERMINATED rồi ATTEMPT_SUBMITTED được gom vào
// events, chỉ gửi sau commit. Trả attempt đã đọc lại sau khi chốt.
func (s *service) terminateAttempt(ctx context.Context, repo repository.Repository, attempt *model.Attempt, reason model.IntegrityTerminationReason, actor string, events *[]pendingEvent) (*model.Attempt, error) {
	*events = append(*events, pendingEvent{userID: attempt.UserID, payload: socket.AttemptEvent{
		Type:    socket.EventAttemptTerminated,
		Payload: socket.AttemptTerminatedPayload{AttemptID: attempt.ID, Reason: string(reason)},
	}})
	termination := &attemptTermination{reason: reason, actor: actor}
	if _, err := s.withRepoDeferringEvents(repo, events).finishAttempt(ctx, attempt.UserID, attempt.ID, nil, termination, ""); err != nil {
		return nil, err
	}
	s.integrity.clearAttempt(attempt.ID)
	return repo.GetAttemptForIntegrity(ctx, attempt.ID)
}
