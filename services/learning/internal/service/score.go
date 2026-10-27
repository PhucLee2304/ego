package service

import (
	"context"

	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

// applyScore là chỗ duy nhất ghi điểm cuối của attempt (PLAN_ATTEMPT_INTEGRITY.md
// mục 1.2): RawScore luôn là điểm chấm từ đáp án; bài bị đình chỉ có Score = 0.
// Mọi luồng ghi điểm (nộp, auto-submit, chấm lại, đình chỉ, khôi phục) đi qua đây.
func applyScore(attempt *model.Attempt, correctAnswers int, rawScore float64) {
	attempt.CorrectAnswers = &correctAnswers
	attempt.RawScore = &rawScore
	score := rawScore
	if attempt.IntegrityOutcome == model.IntegrityOutcomeTerminated {
		score = 0
	}
	attempt.Score = &score
}

// finalizeAttempt chốt attempt ACTIVE sang SUBMITTED đúng một lần: UPDATE có điều
// kiện status = ACTIVE, transaction nào không đổi được dòng thì nhận ErrAttemptNotActive.
func finalizeAttempt(ctx context.Context, repo repository.Repository, attempt *model.Attempt) error {
	attempt.Status = model.AttemptStatusSubmitted
	ok, err := repo.FinalizeAttempt(ctx, attempt)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAttemptNotActive
	}
	return nil
}
