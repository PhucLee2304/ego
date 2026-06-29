package model

import "gorm.io/gorm"

type Option struct {
	gorm.Model
	Key       OptionKey `gorm:"type:text;not null"`
	Content   *string   `gorm:"type:text"`
	IsCorrect bool      `gorm:"not null;default:false"`
	Order     int       `gorm:"not null"`

	QuestionID uint     `gorm:"not null;index"`
	Question   Question `gorm:"foreignKey:QuestionID;references:ID"`
}

type OptionKey string

const (
	OptionA OptionKey = "A"
	OptionB OptionKey = "B"
	OptionC OptionKey = "C"
	OptionD OptionKey = "D"
)
