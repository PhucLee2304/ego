package socket

import (
	"context"

	platformsocket "ego/platform/socket"
	"ego/services/exams/internal/service"
)

func RegisterHandlers(server *platformsocket.Server, svc service.Service) {
	server.Handle(EventAttemptAnswerUpdated, platformsocket.JSONHandler(func(ctx context.Context, client *platformsocket.Client, payload AttemptAnswerUpdatedPayload) (any, error) {
		return handleAttemptAnswerUpdated(ctx, client, svc, payload)
	}))
}
