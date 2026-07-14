package model

import (
	"time"

	"gorm.io/gorm"
)

type AttemptAnswer struct {
	gorm.Model
	IsCorrect  *bool      `gorm:""`
	AnsweredAt *time.Time `gorm:"index"`

	AttemptID uint    `gorm:"not null;index:idx_attempt_question,unique;index"`
	Attempt   Attempt `gorm:"foreignKey:AttemptID;references:ID"`

	QuestionID uint     `gorm:"not null;index:idx_attempt_question,unique;index"`
	Question   Question `gorm:"foreignKey:QuestionID;references:ID"`

	SelectedOptionID *uint   `gorm:"index"`
	SelectedOption   *Option `gorm:"foreignKey:SelectedOptionID;references:ID"`
}
