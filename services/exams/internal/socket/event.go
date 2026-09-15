package socket

import platformsocket "ego/platform/socket"

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
