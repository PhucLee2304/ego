package model

import (
	"gorm.io/gorm"
)

type Exam struct {
	gorm.Model
	Title       string   `gorm:"not null"`
	Description *string  `gorm:"type:text"`
	IsPublic    bool     `gorm:"not null;default:true"`
	Type        ExamType `gorm:"type:text;not null;default:'THPT';index"`
	Year        *int

	Sections []Section `gorm:"foreignKey:ExamID;references:ID"`
}

type ExamType string

const (
	ExamTypeTHPT  ExamType = "THPT"
	ExamTypeTOEIC ExamType = "TOEIC"
)
