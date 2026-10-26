package socket

import platformsocket "ego/platform/socket"

// Luồng TOEIC_STRICT (PLAN_TOEIC_ATTEMPT.md mục 1.3–1.5): client gửi ATTEMPT_LISTENING_ADVANCE
// khi phát xong audio và hết gapMs; ACK mang dto.ListeningStateResponse.
const (
	EventAttemptListeningAdvance platformsocket.MessageType = "ATTEMPT_LISTENING_ADVANCE"
)

const (
	ErrorCodeQuestionLocked           platformsocket.ErrorCode = "QUESTION_LOCKED"
	ErrorCodePageLocked               platformsocket.ErrorCode = "PAGE_LOCKED"
	ErrorCodeListeningPageMismatch    platformsocket.ErrorCode = "LISTENING_PAGE_MISMATCH"
	ErrorCodeListeningAdvanceTooEarly platformsocket.ErrorCode = "LISTENING_ADVANCE_TOO_EARLY"
)

type AttemptListeningAdvancePayload struct {
	AttemptID uint `json:"attemptId" validate:"required,gt=0"`
	Page      int  `json:"page" validate:"gte=0"`
}
