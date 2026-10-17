package socket

import (
	"time"

	"github.com/google/uuid"
)

const (
	EventGradebookUpdated              = "GRADEBOOK_UPDATED"
	EventRoomStarted                   = "ROOM_STARTED"
	EventRoomEnded                     = "ROOM_ENDED"
	EventParticipantPermissionsUpdated = "PARTICIPANT_PERMISSIONS_UPDATED"
	EventHandRaised                    = "HAND_RAISED"
	EventHandLowered                   = "HAND_LOWERED"
	EventScreenShareRequested          = "SCREEN_SHARE_REQUESTED"
	EventScreenShareApproved           = "SCREEN_SHARE_APPROVED"
	EventScreenShareRejected           = "SCREEN_SHARE_REJECTED"
	EventScreenShareStopped            = "SCREEN_SHARE_STOPPED"
	EventParticipantRemoved            = "PARTICIPANT_REMOVED"
	EventParticipantMicrophoneAllowed  = "PARTICIPANT_MICROPHONE_ALLOWED"
)

type RoomEvent struct {
	Type      string    `json:"type"`
	EventID   string    `json:"eventId"`
	RoomID    uint      `json:"roomId"`
	CreatedAt time.Time `json:"createdAt"`
	Payload   any       `json:"payload"`
}

func NewRoomEvent(eventType string, roomID uint, payload any) RoomEvent {
	return RoomEvent{
		Type:      eventType,
		EventID:   uuid.NewString(),
		RoomID:    roomID,
		CreatedAt: time.Now().UTC(),
		Payload:   payload,
	}
}

type GradebookUpdatedEvent struct {
	Type    string                  `json:"type"`
	Payload GradebookUpdatedPayload `json:"payload"`
}

type GradebookUpdatedPayload struct {
	ClassroomID  uint      `json:"classroomId"`
	AssignmentID uint      `json:"assignmentId"`
	StudentID    string    `json:"studentId"`
	AttemptID    uint      `json:"attemptId"`
	Status       string    `json:"status"`
	Score        *float64  `json:"score"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
