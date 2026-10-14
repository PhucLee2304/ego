package media

import (
	"strings"
	"testing"
)

func TestRoomName(t *testing.T) {
	if got := RoomName(42); got != "classroom-room-42" {
		t.Fatalf("RoomName(42) = %q", got)
	}
}

func TestCreateJoinToken(t *testing.T) {
	client := New("http://livekit:7880", "devkey", "local-development-secret-key-000001")
	token, err := client.CreateJoinToken("classroom-room-7", "student-1", "Student One", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("expected JWT token, got %q", token)
	}
}
