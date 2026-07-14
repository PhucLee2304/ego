package socket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"ego/platform/logger"

	"github.com/gorilla/websocket"
)

type Client struct {
	server *Server
	conn   *websocket.Conn
	userID string
	send   chan any
	mu     sync.RWMutex
	once   sync.Once
	done   chan struct{}
	closed bool
}

func newClient(server *Server, conn *websocket.Conn, userID string) *Client {
	return &Client{
		server: server,
		conn:   conn,
		userID: userID,
		send:   make(chan any, server.sendBuffer),
		done:   make(chan struct{}),
	}
}

func (c *Client) UserID() string {
	return c.userID
}

func (c *Client) Send(payload any) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}

	select {
	case c.send <- payload:
		c.mu.RUnlock()
	default:
		c.mu.RUnlock()
		logger.Log.Warn().Str("userID", c.userID).Msg("[SOCKET] Send buffer full, closing client")
		c.Close()
	}
}

func (c *Client) SendAck(requestID string, payload any) {
	if payload == nil {
		c.Send(Ack{
			Type:      MessageTypeAck,
			RequestID: requestID,
			Success:   true,
		})
		return
	}

	c.Send(AckWithPayload{
		Type:      MessageTypeAck,
		RequestID: requestID,
		Success:   true,
		Payload:   payload,
	})
}

func (c *Client) SendError(requestID string, code ErrorCode, message string) {
	c.Send(Error{
		Type:      MessageTypeError,
		RequestID: requestID,
		Success:   false,
		Code:      string(code),
		Message:   message,
	})
}

func (c *Client) Close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		close(c.done)
		close(c.send)
		c.mu.Unlock()

		c.server.unregister(c)
		_ = c.conn.Close()
		logger.Log.Info().Str("userID", c.userID).Msg("[SOCKET] Client disconnected")
	})
}

func (c *Client) readPump(parent context.Context) {
	defer c.Close()

	c.conn.SetReadLimit(c.server.readLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.server.pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.server.pongWait))
	})

	for {
		select {
		case <-parent.Done():
			return
		default:
		}

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Log.Warn().Err(err).Str("userID", c.userID).Msg("[SOCKET] Unexpected close")
			}
			return
		}

		var message Message
		if err := json.Unmarshal(data, &message); err != nil {
			c.SendError("", ErrorCodeInvalidJSON, "invalid JSON message")
			continue
		}

		c.server.dispatch(parent, c, message)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(c.server.pingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				_ = writeClose(c.conn, c.server.writeWait)
				return
			}
			if err := writeJSON(c.conn, payload, c.server.writeWait); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.server.writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
