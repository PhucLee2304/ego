package model

import (
	"time"

	"gorm.io/gorm"
)

// IntegrityPolicySnapshot là cấu hình giám sát chụp vào attempt lúc tạo.
type IntegrityPolicySnapshot struct {
	MaxViolations   int    `json:"maxViolations"`
	TakeoverDelayMs int    `json:"takeoverDelayMs"`
	ConsentVersion  string `json:"consentVersion"`
}

type IntegrityOutcome string

const (
	IntegrityOutcomeNone       IntegrityOutcome = "NONE"
	IntegrityOutcomeTerminated IntegrityOutcome = "TERMINATED"
	IntegrityOutcomeReinstated IntegrityOutcome = "REINSTATED"
)

type IntegrityTerminationReason string

const (
	IntegrityTerminationMaxViolations IntegrityTerminationReason = "MAX_VIOLATIONS"
	IntegrityTerminationTeacherBanned IntegrityTerminationReason = "TEACHER_BANNED"
)

type IntegrityLevel string

const (
	IntegrityLevelLog       IntegrityLevel = "LOG"
	IntegrityLevelWarning   IntegrityLevel = "WARNING"
	IntegrityLevelViolation IntegrityLevel = "VIOLATION"
)

type IntegrityEventType string

const (
	IntegrityEventAwayStart         IntegrityEventType = "AWAY_START"
	IntegrityEventAwayEnd           IntegrityEventType = "AWAY_END"
	IntegrityEventReload            IntegrityEventType = "RELOAD"
	IntegrityEventNavigationBlocked IntegrityEventType = "NAVIGATION_BLOCKED"
	IntegrityEventBackBlocked       IntegrityEventType = "BACK_BLOCKED"
	IntegrityEventCopy              IntegrityEventType = "COPY"
	IntegrityEventPaste             IntegrityEventType = "PASTE"
	IntegrityEventSessionConflict   IntegrityEventType = "SESSION_CONFLICT"
	IntegrityEventSessionTakeover   IntegrityEventType = "SESSION_TAKEOVER"
	IntegrityEventSessionReleased   IntegrityEventType = "SESSION_RELEASED"
	IntegrityEventTeacherBanned     IntegrityEventType = "TEACHER_BANNED"
	IntegrityEventContinueAllowed   IntegrityEventType = "CONTINUE_ALLOWED"
	IntegrityEventResultReinstated  IntegrityEventType = "RESULT_REINSTATED"
)

type IntegrityReview string

const (
	IntegrityReviewContinueAllowed IntegrityReview = "CONTINUE_ALLOWED"
	IntegrityReviewBanned          IntegrityReview = "BANNED"
)

// AttemptIntegrityEvent là một sự kiện giám sát; EventID unique theo attempt (idempotency).
type AttemptIntegrityEvent struct {
	gorm.Model
	AttemptID    uint               `gorm:"not null;index"`
	EventID      string             `gorm:"type:text;not null"`
	SessionEpoch int                `gorm:"not null;default:0"`
	ConnectionID *string            `gorm:"type:text"`
	Type         IntegrityEventType `gorm:"type:text;not null"`
	Level        IntegrityLevel     `gorm:"type:text;not null;index"`
	ViolationNo  *int
	StartEventID *string `gorm:"type:text"`

	ServerDurationMs *int
	ClientDurationMs *int
	Replayed         bool    `gorm:"not null;default:false"`
	Stale            bool    `gorm:"not null;default:false"`
	Detail           *string `gorm:"type:text"`

	ClientAt   *time.Time
	ReceivedAt time.Time `gorm:"not null"`

	Review     *IntegrityReview `gorm:"type:text"`
	ReviewedAt *time.Time
	ReviewedBy *string `gorm:"type:text"`
}

// AttemptIntegrityEventRow là event kèm học sinh của attempt (kết quả join, không phải bảng).
type AttemptIntegrityEventRow struct {
	AttemptIntegrityEvent `gorm:"embedded"`
	StudentID             string
}

// AttemptIntegritySummary là tóm tắt giám sát của một attempt cho gradebook.
type AttemptIntegritySummary struct {
	AttemptID         uint
	Proctored         bool
	ViolationCount    int
	PendingViolations int
	IntegrityOutcome  IntegrityOutcome
	TerminationReason *IntegrityTerminationReason
}
