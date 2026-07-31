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
}

type HandlerError struct {
	Code    ErrorCode
	Message string
}

func (e *HandlerError) Error() string {
	return e.Message
}

func NewHandlerError(code ErrorCode, message string) *HandlerError {
	return &HandlerError{Code: code, Message: message}
}
