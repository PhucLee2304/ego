package socket

import (
	"context"

	platformsocket "ego/platform/socket"
)

type AttemptAnswerUpdatedPayload struct {
	AttemptID  uint  `json:"attemptId"`
	QuestionID uint  `json:"questionId"`
	OptionID   *uint `json:"optionId,omitempty"`
}

func handleAttemptAnswerUpdated(_ context.Context, _ *platformsocket.Client, _ AttemptAnswerUpdatedPayload) (any, error) {
	// TODO: Implement exams autosave business logic:
	// 1. Validate attempt ownership by current user.
	// 2. Ensure attempt is still active.
	// 3. Validate question belongs to the attempt/exam scope.
	// 4. Upsert selected answer into attempt_answers.
	// 5. Return ack payload if the client later needs sync metadata.
	return nil, nil
}
