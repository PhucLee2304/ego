package socket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestOnDisconnect(t *testing.T) {
	server := NewServer(nil)
	var got []string
	server.OnDisconnect(func(c *Client) {
		// Hook chạy ngoài khóa nên dùng lại server được.
		server.BroadcastToUser(c.UserID(), "ping")
		got = append(got, c.ID())
	})

	first := newClient(server, nil, "u1")
	second := newClient(server, nil, "u1")
	if first.ID() == "" || first.ID() == second.ID() {
		t.Fatalf("ID phải khác rỗng và riêng từng kết nối: %q %q", first.ID(), second.ID())
	}
	server.register(first)
	server.register(second)

	server.unregister(first)
	if len(got) != 1 || got[0] != first.ID() {
		t.Fatalf("hook nhận %v, want [%s]", got, first.ID())
	}
	if _, ok := server.clients["u1"][second]; !ok {
		t.Fatal("kết nối còn lại bị gỡ nhầm")
	}
}

func TestDisconnectUser(t *testing.T) {
	server := NewServer(nil)
	// Bỏ qua bước xác thực: httptest upgrade rồi gắn user theo query ?user=.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := server.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		client := newClient(server, conn, r.URL.Query().Get("user"))
		server.register(client)
		go client.writePump()
		client.readPump(r.Context())
	}))
	defer ts.Close()

	dial := func(user string) *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"?user="+user, nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}
	blocked1, blocked2, other := dial("u1"), dial("u1"), dial("u2")
	defer other.Close()

	// Chờ server register đủ 3 kết nối.
	for i := 0; i < 100 && server.connectionCount() < 3; i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if n := server.DisconnectUser("u1"); n != 2 {
		t.Fatalf("DisconnectUser đóng %d kết nối, want 2", n)
	}
	for _, conn := range []*websocket.Conn{blocked1, blocked2} {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err := conn.ReadMessage()
		if !websocket.IsCloseError(err, CloseCodeUserBlocked) {
			t.Fatalf("err=%v, want close code %d", err, CloseCodeUserBlocked)
		}
	}
	if server.connectionCount() != 1 {
		t.Fatalf("còn %d kết nối, want 1 (của u2)", server.connectionCount())
	}
	if n := server.DisconnectUser("u1"); n != 0 {
		t.Fatalf("gọi lại DisconnectUser đóng %d, want 0", n)
	}
}

func (s *Server) connectionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, clients := range s.clients {
		total += len(clients)
	}
	return total
}
