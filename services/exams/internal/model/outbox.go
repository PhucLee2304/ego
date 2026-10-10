package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type OutboxEvent struct {
	gorm.Model
	Type          OutboxEventType   `gorm:"type:text;not null;index"`
	AggregateID   *string           `gorm:"type:text;index"`
	Status        OutboxEventStatus `gorm:"type:text;not null;default:PENDING;index"`
	Payload       datatypes.JSON    `gorm:"type:jsonb;not null"`
	Attempts      int               `gorm:"not null;default:0"`
	NextAttemptAt time.Time         `gorm:"not null;index"`
	ProcessedAt   *time.Time        `gorm:"index"`
	LastError     *string           `gorm:"type:text"`
}

type OutboxEventType string

const (
	OutboxEventTypeClassroomAssignmentAttemptCreated   OutboxEventType = "CLASSROOM_ASSIGNMENT_ATTEMPT_CREATED"
	OutboxEventTypeClassroomAssignmentAttemptSubmitted OutboxEventType = "CLASSROOM_ASSIGNMENT_ATTEMPT_SUBMITTED"
)

type OutboxEventStatus string

const (
	OutboxEventStatusPending         OutboxEventStatus = "PENDING"
	OutboxEventStatusProcessing      OutboxEventStatus = "PROCESSING"
	OutboxEventStatusProcessed       OutboxEventStatus = "PROCESSED"
	OutboxEventStatusFailed          OutboxEventStatus = "FAILED"
	OutboxEventStatusFailedPermanent OutboxEventStatus = "FAILED_PERMANENT"
)
