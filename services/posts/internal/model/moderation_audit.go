package model

import "time"

type ModerationAction string

const (
	ModerationHide   ModerationAction = "HIDE"
	ModerationUnhide ModerationAction = "UNHIDE"
	ModerationDelete ModerationAction = "DELETE"
)

type PostModerationAudit struct {
	ID          uint             `gorm:"primaryKey"`
	PostID      uint             `gorm:"not null;index"`
	AdminUserID string           `gorm:"type:text;not null;index"`
	Action      ModerationAction `gorm:"type:text;not null"`
	Reason      string           `gorm:"type:text;not null"`
	CreatedAt   time.Time
}
