package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExamAudit struct {
	gorm.Model
	ExamID     uint           `gorm:"not null;index"`
	ActorID    string         `gorm:"type:text;not null;index"`
	Reason     string         `gorm:"type:text;not null"`
	Before     datatypes.JSON `gorm:"type:jsonb;not null"`
	After      datatypes.JSON `gorm:"type:jsonb;not null"`
	RegradedAt *time.Time     `gorm:"index"`
}
