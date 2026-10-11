package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type NotificationOutbox struct {
	gorm.Model
	EventKey      string                      `gorm:"type:text;not null;uniqueIndex"`
	Recipients    datatypes.JSONSlice[string] `gorm:"type:jsonb;not null"`
	Title         string                      `gorm:"type:text;not null"`
	Body          string                      `gorm:"type:text;not null"`
	Data          datatypes.JSONMap           `gorm:"type:jsonb;not null;default:'{}'"`
	Attempts      int                         `gorm:"not null;default:0"`
	NextAttemptAt time.Time                   `gorm:"not null;index"`
	ProcessedAt   *time.Time                  `gorm:"index"`
	LastError     *string                     `gorm:"type:text"`
}
