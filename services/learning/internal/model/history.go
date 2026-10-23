package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type History struct {
	gorm.Model
	Data datatypes.JSON `gorm:"type:jsonb;not null"`

	AttemptID uint    `gorm:"not null;uniqueIndex"`
	Attempt   Attempt `gorm:"foreignKey:AttemptID;references:ID"`

	ExamTitle string   `gorm:"not null"`
	ExamType  ExamType `gorm:"type:text;not null;index"`
	ExamYear  *int
}
