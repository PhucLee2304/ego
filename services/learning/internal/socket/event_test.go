package socket

import "testing"

func TestNewRoomEventCreatesUniqueEventIDs(t *testing.T) {
	first := NewRoomEvent(EventParticipantPermissionsUpdated, 7, map[string]bool{"enabled": true})
	second := NewRoomEvent(EventParticipantPermissionsUpdated, 7, map[string]bool{"enabled": true})

	if first.EventID == "" || second.EventID == "" {
		t.Fatal("event ID must not be empty")
	}
	if first.EventID == second.EventID {
		t.Fatalf("event IDs must be unique: %q", first.EventID)
	}
	if first.RoomID != 7 || first.Type != EventParticipantPermissionsUpdated || first.CreatedAt.IsZero() {
		t.Fatalf("unexpected room event: %#v", first)
	}
}
