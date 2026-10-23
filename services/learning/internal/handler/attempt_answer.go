package handler

import (
	"context"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/socket"
)

// RegisterSocketHandlers gắn handler message của hub /ws/exams. Đặt ở handler
// (không ở socket) vì service đã import socket.
func (h *handler) RegisterSocketHandlers(examsHub *platformsocket.Server) {
	examsHub.Handle(socket.EventAttemptAnswerUpdated, platformsocket.JSONHandler(func(ctx context.Context, client *platformsocket.Client, payload socket.AttemptAnswerUpdatedPayload) (any, error) {
		// Service đã trả lỗi kèm mã WS (ATTEMPT_NOT_FOUND, ATTEMPT_EXPIRED…).
		return h.service.UpdateAttemptAnswer(ctx, client.UserID(), payload.AttemptID, payload.QuestionID, payload.OptionID)
	}))
}
