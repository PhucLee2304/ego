package model

import (
	"time"

	"gorm.io/gorm"
)

type Attempt struct {
	gorm.Model
	Mode           AttemptMode   `gorm:"type:text;not null;index"`
	Status         AttemptStatus `gorm:"type:text;not null;index"`
	SectionCode    *SectionCode  `gorm:"type:text;index"`
	PartCode       *PartCode     `gorm:"type:text;index"`
	StartedAt      time.Time     `gorm:"not null"`
	ExpiresAt      *time.Time    `gorm:"index"`
	SubmittedAt    *time.Time
	Duration       *int
	TotalQuestions int `gorm:"not null;default:0"`
	CorrectAnswers *int
	Score          *float64

	ExamID uint `gorm:"not null;index"`
	Exam   Exam `gorm:"foreignKey:ExamID;references:ID"`

	UserID string `gorm:"type:text;not null;index"`

	Answers []AttemptAnswer `gorm:"foreignKey:AttemptID;references:ID"`
	History *History        `gorm:"foreignKey:AttemptID;references:ID"`
}

type AttemptMode string

const (
	AttemptModePractice AttemptMode = "PRACTICE"
	AttemptModeTest     AttemptMode = "TEST"
)

type AttemptStatus string

const (
	AttemptStatusInProgress AttemptStatus = "ACTIVE"
	AttemptStatusSubmitted  AttemptStatus = "SUBMITTED"
	AttemptStatusCancelled  AttemptStatus = "CANCELLED"
)
