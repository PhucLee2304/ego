package socket

import platformsocket "ego/platform/socket"

func RegisterHandlers(server *platformsocket.Server) {
	server.Handle(EventAttemptAnswerUpdated, platformsocket.JSONHandler(handleAttemptAnswerUpdated))
}
