package socket

import platformsocket "ego/platform/socket"

// Hub /ws/exams: client gửi ATTEMPT_ANSWER_UPDATED, server trả ACK hoặc ERROR
// với một trong các mã dưới đây (giữ nguyên từ exams).
const (
	EventAttemptAnswerUpdated platformsocket.MessageType = "ATTEMPT_ANSWER_UPDATED"
)

const (
	ErrorCodeAttemptNotFound       platformsocket.ErrorCode = "ATTEMPT_NOT_FOUND"
	ErrorCodeAttemptNotActive      platformsocket.ErrorCode = "ATTEMPT_NOT_ACTIVE"
	ErrorCodeAttemptExpired        platformsocket.ErrorCode = "ATTEMPT_EXPIRED"
	ErrorCodeQuestionNotInAttempt  platformsocket.ErrorCode = "QUESTION_NOT_IN_ATTEMPT"
	ErrorCodeOptionNotInQuestion   platformsocket.ErrorCode = "OPTION_NOT_IN_QUESTION"
	ErrorCodeAttemptAnswerRejected platformsocket.ErrorCode = "ATTEMPT_ANSWER_REJECTED"
)

type AttemptAnswerUpdatedPayload struct {
	AttemptID  uint  `json:"attemptId" validate:"required,gt=0"`
	QuestionID uint  `json:"questionId" validate:"required,gt=0"`
	OptionID   *uint `json:"optionId,omitempty"`
}
