package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Notification struct {
	gorm.Model
	UserID         string            `gorm:"type:text;not null;index"`
	Title          string            `gorm:"type:text;not null"`
	Body           string            `gorm:"type:text;not null"`
	SourceService  string            `gorm:"type:text;not null;index"`
	Data           datatypes.JSONMap `gorm:"type:jsonb;not null;default:'{}'"`
	IdempotencyKey string            `gorm:"type:text;not null;uniqueIndex"`
	ReadAt         *time.Time        `gorm:"index"`
}
