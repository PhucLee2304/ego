package socket

import (
	"context"
	tokenClient "ego/api/gen/go/token"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"ego/platform/httpx"
	"ego/platform/logger"

	"github.com/gorilla/websocket"
)

type HandlerFunc func(ctx context.Context, client *Client, message Message) (any, error)

type Server struct {
	tokenServiceClient tokenClient.TokenServiceClient
	upgrader      websocket.Upgrader
	writeWait     time.Duration
	pongWait      time.Duration
	pingInterval  time.Duration
	readLimit     int64
	sendBuffer    int

	mu       sync.RWMutex
	handlers map[MessageType]HandlerFunc
	clients  map[string]map[*Client]struct{}
}

func NewServer(tokenServiceClient tokenClient.TokenServiceClient) *Server {
	return &Server{
		tokenServiceClient: tokenServiceClient,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		writeWait:    10 * time.Second,
		pongWait:     60 * time.Second,
		pingInterval: 45 * time.Second,
		readLimit:    64 * 1024,
		sendBuffer:   256,
		handlers:     make(map[MessageType]HandlerFunc),
		clients:      make(map[string]map[*Client]struct{}),
	}
}

func (s *Server) Handle(messageType MessageType, handler HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[messageType] = handler
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.tokenServiceClient == nil {
		httpx.Error(w, http.StatusInternalServerError, "[SOCKET] Token service client is not configured")
		return
	}

	userID, err := AuthenticateToken(r.Context(), r, s.tokenServiceClient)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, fmt.Sprintf("[SOCKET] Unauthorized: %v", err))
		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Log.Error().Err(err).Msgf("[SOCKET] Failed to upgrade connection: %v", err)
		return
	}

	client := newClient(s, conn, userID)
	s.register(client)

	logger.Log.Info().Str("userID", userID).Msg("[SOCKET] Client connected")
	go client.writePump()
	go client.readPump(r.Context())
}

func (s *Server) BroadcastToUser(userID string, payload any) {
	s.mu.RLock()
	clients := make([]*Client, 0, len(s.clients[userID]))
	for client := range s.clients[userID] {
		clients = append(clients, client)
	}
	s.mu.RUnlock()

	for _, client := range clients {
		client.Send(payload)
	}
}

func (s *Server) register(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clients[client.userID] == nil {
		s.clients[client.userID] = make(map[*Client]struct{})
	}
	s.clients[client.userID][client] = struct{}{}
}

func (s *Server) unregister(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userClients, ok := s.clients[client.userID]; ok {
		delete(userClients, client)
		if len(userClients) == 0 {
			delete(s.clients, client.userID)
		}
	}
}

func (s *Server) dispatch(ctx context.Context, client *Client, message Message) {
	if message.Type == "" {
		client.SendError(message.RequestID, ErrorCodeInvalidMessage, "message type is required")
		return
	}

	message.RequestID = strings.TrimSpace(message.RequestID)
	if message.RequestID == "" {
		client.SendError("", ErrorCodeInvalidMessage, "requestId is required")
		return
	}

	s.mu.RLock()
	handler := s.handlers[message.Type]
	s.mu.RUnlock()

	if handler == nil {
		client.SendError(message.RequestID, ErrorCodeUnknownMessageType, "unknown message type")
		return
	}

	payload, err := handler(ctx, client, message)
	if err != nil {
		var handlerErr *HandlerError
		if errors.As(err, &handlerErr) {
			client.SendError(message.RequestID, handlerErr.Code, handlerErr.Message)
			return
		}

		logger.Log.Error().Err(err).Str("type", string(message.Type)).Msgf("[SOCKET] Handler failed: %v", err)
		client.SendError(message.RequestID, ErrorCodeInternalError, fmt.Sprintf("failed to process message: %v", err))
		return
	}

	client.SendAck(message.RequestID, payload)
}

func writeJSON(conn *websocket.Conn, payload any, writeWait time.Duration) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return conn.WriteJSON(payload)
}

func writeClose(conn *websocket.Conn, writeWait time.Duration) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.CloseMessage, []byte{})
}
