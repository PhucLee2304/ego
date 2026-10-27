package socket

import (
	"time"

	platformsocket "ego/platform/socket"
)

// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 1.9). Client gửi qua /ws/exams.
const (
	EventAttemptSessionClaim   platformsocket.MessageType = "ATTEMPT_SESSION_CLAIM"
	EventAttemptIntegrityEvent platformsocket.MessageType = "ATTEMPT_INTEGRITY_EVENT"
)

const (
	ErrorCodeAttemptNotProctored     platformsocket.ErrorCode = "ATTEMPT_NOT_PROCTORED"
	ErrorCodeSessionInUse            platformsocket.ErrorCode = "SESSION_IN_USE"
	ErrorCodeSessionRecovering       platformsocket.ErrorCode = "SESSION_RECOVERING"
	ErrorCodeSessionTakeoverRequired platformsocket.ErrorCode = "SESSION_TAKEOVER_REQUIRED"
	ErrorCodeIntegrityEventInvalid   platformsocket.ErrorCode = "INTEGRITY_EVENT_INVALID"
)

// Push tới học sinh (/ws/exams) và giáo viên (/ws/classrooms).
const (
	EventAttemptViolation  = "ATTEMPT_VIOLATION"
	EventAttemptTerminated = "ATTEMPT_TERMINATED"
	// Giáo viên cho tiếp tục làm bài: vi phạm được xóa, ViolationCount giảm 1.
	EventAttemptViolationCleared = "ATTEMPT_VIOLATION_CLEARED"
	EventAttemptSessionRevoked   = "ATTEMPT_SESSION_REVOKED"
	EventAttemptReset            = "ATTEMPT_RESET"
	EventAttemptIntegrity        = "ATTEMPT_INTEGRITY_UPDATED"
)

type AttemptSessionClaimPayload struct {
	AttemptID    uint   `json:"attemptId" validate:"required,gt=0"`
	SessionToken string `json:"sessionToken,omitempty"`
	Takeover     bool   `json:"takeover,omitempty"`
}

type AttemptIntegrityEventPayload struct {
	AttemptID        uint       `json:"attemptId" validate:"required,gt=0"`
	EventID          string     `json:"eventId" validate:"required,notblank,max=64"`
	SessionEpoch     int        `json:"sessionEpoch"`
	Type             string     `json:"type" validate:"required"`
	At               *time.Time `json:"at,omitempty"`
	Replayed         bool       `json:"replayed,omitempty"`
	StartEventID     *string    `json:"startEventId,omitempty" validate:"omitempty,max=64"`
	ClientDurationMs *int       `json:"clientDurationMs,omitempty" validate:"omitempty,gte=0"`
	Detail           *string    `json:"detail,omitempty" validate:"omitempty,max=500"`
}

// AttemptEvent là push chung tới học sinh: {type, payload}.
type AttemptEvent struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type AttemptViolationPayload struct {
	AttemptID      uint   `json:"attemptId"`
	EventID        string `json:"eventId"`
	ViolationCount int    `json:"violationCount"`
	MaxViolations  int    `json:"maxViolations"`
}

type AttemptTerminatedPayload struct {
	AttemptID uint   `json:"attemptId"`
	Reason    string `json:"reason"`
}

type AttemptSessionRevokedPayload struct {
	AttemptID    uint `json:"attemptId"`
	SessionEpoch int  `json:"sessionEpoch"`
}

type AttemptResetPayload struct {
	AttemptID uint `json:"attemptId"`
}

type AttemptIntegrityUpdatedEvent struct {
	Type    string                         `json:"type"`
	Payload AttemptIntegrityUpdatedPayload `json:"payload"`
}

type AttemptIntegrityUpdatedPayload struct {
	ClassroomID    uint                    `json:"classroomId"`
	AssignmentID   uint                    `json:"assignmentId"`
	StudentID      string                  `json:"studentId"`
	AttemptID      uint                    `json:"attemptId"`
	Event          *AttemptIntegrityRecord `json:"event,omitempty"`
	ViolationCount int                     `json:"violationCount"`
	MaxViolations  int                     `json:"maxViolations"`
	Status         string                  `json:"status"`
	Outcome        string                  `json:"outcome"`
	Away           bool                    `json:"away"`
}

type AttemptIntegrityRecord struct {
	EventID          string    `json:"eventId"`
	Type             string    `json:"type"`
	Level            string    `json:"level"`
	ViolationNo      *int      `json:"violationNo,omitempty"`
	ServerDurationMs *int      `json:"serverDurationMs,omitempty"`
	At               time.Time `json:"at"`
	Review           *string   `json:"review,omitempty"`
}
