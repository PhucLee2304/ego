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
	examsHub.Handle(socket.EventAttemptListeningAdvance, platformsocket.JSONHandler(func(ctx context.Context, client *platformsocket.Client, payload socket.AttemptListeningAdvancePayload) (any, error) {
		return h.service.AdvanceListening(ctx, client.UserID(), payload.AttemptID, payload.Page)
	}))
	// Giám sát: kết nối (client.ID) quyết định ai là chủ phiên làm bài.
	examsHub.Handle(socket.EventAttemptSessionClaim, platformsocket.JSONHandler(func(ctx context.Context, client *platformsocket.Client, payload socket.AttemptSessionClaimPayload) (any, error) {
		return h.service.ClaimAttemptSession(ctx, client.UserID(), client.ID(), payload)
	}))
	examsHub.Handle(socket.EventAttemptIntegrityEvent, platformsocket.JSONHandler(func(ctx context.Context, client *platformsocket.Client, payload socket.AttemptIntegrityEventPayload) (any, error) {
		return h.service.RecordIntegrityEvent(ctx, client.UserID(), client.ID(), payload)
	}))
	examsHub.OnDisconnect(func(client *platformsocket.Client) {
		h.service.ReleaseAttemptConnection(context.Background(), client.ID())
	})
}
