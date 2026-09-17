package model

import (
	"time"

	"gorm.io/gorm"
)

type Event struct {
	gorm.Model
	ClassroomID uint      `gorm:"not null;index"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	CreatedBy           string      `gorm:"type:text;not null;index"`
	Type                EventType   `gorm:"type:text;not null;index"`
	Title               string      `gorm:"type:text;not null"`
	Description         *string     `gorm:"type:text"`
	StartsAt            time.Time   `gorm:"not null;index"`
	EndsAt              *time.Time  `gorm:"index"`
	Location            *string     `gorm:"type:text"`
	MeetingURL          *string     `gorm:"type:text"`
	RelatedAssignmentID *uint       `gorm:"index"`
	RelatedAssignment   *Assignment `gorm:"foreignKey:RelatedAssignmentID;references:ID"`
}

type EventType string

const (
	EventTypeClassSession EventType = "CLASS_SESSION"
	EventTypeDeadline     EventType = "DEADLINE"
	EventTypeExamWindow   EventType = "EXAM_WINDOW"
	EventTypeCustom       EventType = "CUSTOM"
)
