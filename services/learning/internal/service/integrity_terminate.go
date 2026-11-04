package service

import (
	"context"
	"fmt"
	"time"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/datatypes"
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

// banSubmittedAttempt: giáo viên Cấm bài đã nộp (phát hiện muộn). Score về 0, RawScore giữ
// điểm đạt được để Khôi phục sau; history, submission, gradebook và thông báo cập nhật cùng
// tx như ReinstateAttempt. Bài đang bị đình chỉ thì trả ErrAttemptAlreadyTerminated.
func (s *service) banSubmittedAttempt(ctx context.Context, repo repository.Repository, locked *model.Attempt, teacherID string) (*model.Attempt, *gradebookEvent, error) {
	if locked.IntegrityOutcome == model.IntegrityOutcomeTerminated {
		return nil, nil, ErrAttemptAlreadyTerminated
	}
	attempt, err := repo.GetAttemptForLifecycle(ctx, locked.ID, locked.UserID)
	if err != nil {
		return nil, nil, err
	}
	now := s.clock()
	reason := model.IntegrityTerminationTeacherBanned
	attempt.IntegrityOutcome = model.IntegrityOutcomeTerminated
	attempt.TerminatedAt, attempt.TerminationReason, attempt.TerminatedBy = &now, &reason, &teacherID
	correct, raw := 0, 0.0
	if attempt.CorrectAnswers != nil {
		correct = *attempt.CorrectAnswers
	}
	switch {
	case attempt.RawScore != nil:
		raw = *attempt.RawScore
	case attempt.Score != nil:
		raw = *attempt.Score
	}
	applyScore(attempt, correct, raw)
	if err := repo.UpdateAttemptIntegrity(ctx, attempt); err != nil {
		return nil, nil, err
	}
	data, err := dto.MarshalAttemptHistory(attempt)
	if err != nil {
		return nil, nil, err
	}
	if err := repo.UpsertHistory(ctx, &model.History{Data: datatypes.JSON(data), AttemptID: attempt.ID, ExamTitle: attempt.Exam.Title, ExamType: attempt.Exam.Type, ExamYear: attempt.Exam.Year}); err != nil {
		return nil, nil, err
	}
	gradebook, err := updateAssignmentSubmissionResult(ctx, repo, attempt)
	if err != nil {
		return nil, nil, err
	}
	if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAttempt, fmt.Sprintf("attempt:%d:banned:%d", attempt.ID, now.UnixNano()), []string{attempt.UserID},
		"Assignment result suspended", "Your teacher suspended the result of your assignment", "ATTEMPT", attempt.ID, 0, time.Time{}); err != nil {
		return nil, nil, err
	}
	return attempt, gradebook, nil
}
