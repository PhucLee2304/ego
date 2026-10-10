package model

import (
	"time"

	"gorm.io/gorm"
)

type Schedule struct {
	gorm.Model
	ClassroomID uint      `gorm:"not null;index"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	Title       string    `gorm:"type:text;not null"`
	Description *string   `gorm:"type:text"`
	StartsAt    time.Time `gorm:"not null;index"`
	EndsAt      time.Time `gorm:"not null;index"`
}
