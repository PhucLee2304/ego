package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/logger"
)

// submitAssignmentAttempts nộp ngay mọi attempt đang làm của một assignment bằng
// đáp án đã lưu (thay RPC SubmitAssignmentAttempts của exams). Gọi trong tx của
// assignment qua withRepoDeferringEvents: mỗi bài nộp là một SAVEPOINT; bài nộp lỗi
// được bỏ qua và để worker auto-submit làm lại vì expires_at đã được đặt về now.
func (s *service) submitAssignmentAttempts(ctx context.Context, assignmentID uint, reason string) (int, error) {
	if err := s.repo.ExpireActiveAssignmentAttempts(ctx, assignmentID, time.Now()); err != nil {
		return 0, err
	}
	attempts, err := s.repo.GetActiveAttemptsByAssignmentID(ctx, assignmentID)
	if err != nil {
		return 0, err
	}

	submitted := 0
	var errs []error
	for _, attempt := range attempts {
		if _, err := s.submitAttemptWithReason(ctx, attempt.UserID, attempt.ID, reason); err != nil {
			if errors.Is(err, ErrAttemptAlreadySubmitted) || errors.Is(err, ErrAttemptCancelled) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		submitted++
	}
	if len(errs) > 0 {
		logger.Log.Error().Err(errors.Join(errs...)).Uint("assignment_id", assignmentID).
			Msg("[ATTEMPT] Some assignment attempts were queued for auto-submit retry")
	}
	return submitted, nil
}

// submitStudentAssignmentAttempts nộp ngay các bài assignment đang làm của một học
// sinh (thay RPC SubmitStudentAssignmentAttempts, dùng khi xóa học sinh khỏi lớp).
// Gọi trong tx qua withRepoDeferringEvents; khác submitAssignmentAttempts, bài nộp
// lỗi làm hỏng cả tx như khi RPC lỗi trước đây.
func (s *service) submitStudentAssignmentAttempts(ctx context.Context, studentID string, assignmentIDs []uint) (int, error) {
	if len(assignmentIDs) == 0 {
		return 0, nil
	}
	if err := s.repo.ExpireActiveAssignmentAttemptsByUser(ctx, assignmentIDs, studentID, time.Now()); err != nil {
		return 0, err
	}
	attempts, err := s.repo.GetActiveAttemptsByAssignmentIDsAndUser(ctx, assignmentIDs, studentID)
	if err != nil {
		return 0, err
	}

	submitted := 0
	var errs []error
	for _, attempt := range attempts {
		if _, err := s.submitAttemptWithReason(ctx, studentID, attempt.ID, SubmitReasonRemovedFromClass); err != nil {
			if errors.Is(err, ErrAttemptAlreadySubmitted) || errors.Is(err, ErrAttemptCancelled) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		submitted++
	}
	if len(errs) > 0 {
		return submitted, errors.Join(errs...)
	}
	return submitted, nil
}
