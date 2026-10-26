package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationSource là SourceService gửi sang notifications, đặt theo domain
// (trùng tên file domain trong service/) đã sinh ra thông báo.
type NotificationSource string

const (
	NotificationSourceExam       NotificationSource = "EXAM"       // correction.go: exam:%d:regrade:…
	NotificationSourceAttempt    NotificationSource = "ATTEMPT"    // submit.go: attempt:%d:submitted
	NotificationSourceClassroom  NotificationSource = "CLASSROOM"  // classroom.go
	NotificationSourceMember     NotificationSource = "MEMBER"     // member.go: classroom:%d:join-request|member:…
	NotificationSourceSchedule   NotificationSource = "SCHEDULE"   // schedule.go
	NotificationSourceRoom       NotificationSource = "ROOM"       // room.go: room:%d:started
	NotificationSourceAssignment NotificationSource = "ASSIGNMENT" // assignment.go: assignment:%d:created|due-reminder
)

type NotificationOutbox struct {
	gorm.Model
	EventKey      string                      `gorm:"type:text;not null;uniqueIndex"`
	Source        NotificationSource          `gorm:"type:text;not null"`
	Recipients    datatypes.JSONSlice[string] `gorm:"type:jsonb;not null"`
	Title         string                      `gorm:"type:text;not null"`
	Body          string                      `gorm:"type:text;not null"`
	Data          datatypes.JSONMap           `gorm:"type:jsonb;not null;default:'{}'"`
	Attempts      int                         `gorm:"not null;default:0"`
	NextAttemptAt time.Time                   `gorm:"not null;index"`
	ProcessedAt   *time.Time                  `gorm:"index"`
	LastError     *string                     `gorm:"type:text"`
}
