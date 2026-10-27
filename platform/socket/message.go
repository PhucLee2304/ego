package socket

import "encoding/json"

type Message struct {
	Type      MessageType     `json:"type"`
	RequestID string          `json:"requestId"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Ack struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"requestId"`
	Success   bool        `json:"success"`
	Payload   any         `json:"payload,omitempty"`
}

type Error struct {
	Type      MessageType `json:"type"`
	RequestID string      `json:"requestId"`
	Success   bool        `json:"success"`
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   any         `json:"details,omitempty"`
}

type HandlerError struct {
	Code    ErrorCode
	Message string
	// Details là dữ liệu máy đọc được kèm lỗi (vd retryAfterMs), gửi trong field details.
	Details any
}

func (e *HandlerError) Error() string {
	return e.Message
}

func NewHandlerError(code ErrorCode, message string) *HandlerError {
	return &HandlerError{Code: code, Message: message}
}

func (e *HandlerError) WithDetails(details any) *HandlerError {
	e.Details = details
	return e
}
