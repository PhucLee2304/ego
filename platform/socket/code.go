package socket

type MessageType string

const (
	MessageTypeAck   MessageType = "ACK"
	MessageTypeError MessageType = "ERROR"
)

type ErrorCode string

const (
	ErrorCodeInvalidMessage     ErrorCode = "INVALID_MESSAGE"
	ErrorCodeInvalidJSON        ErrorCode = "INVALID_JSON"
	ErrorCodeUnknownMessageType ErrorCode = "UNKNOWN_MESSAGE_TYPE"
	ErrorCodeInternalError      ErrorCode = "INTERNAL_ERROR"
	ErrorCodeInvalidPayload     ErrorCode = "INVALID_PAYLOAD"
)
