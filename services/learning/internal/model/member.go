package model

import (
	"time"

	"gorm.io/gorm"
)

type Member struct {
	gorm.Model
	ClassroomID uint      `gorm:"not null;index;uniqueIndex:idx_members_classroom_user"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`
	UserID      string    `gorm:"type:text;not null;index;uniqueIndex:idx_members_classroom_user"`

	Approved   *bool      `gorm:"index"`
	ApprovedAt *time.Time `gorm:"index"`
	JoinedAt   *time.Time `gorm:"index"`
}
