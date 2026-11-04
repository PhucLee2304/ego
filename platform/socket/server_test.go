package socket

import "testing"

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
