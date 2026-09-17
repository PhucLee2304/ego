package model

import (
	"time"

	"gorm.io/gorm"
)

type Assignment struct {
	gorm.Model
	Title       string    `gorm:"type:text;not null"`
	Description *string   `gorm:"type:text"`
	ExamID      uint      `gorm:"not null;index"`
	OpensAt     time.Time `gorm:"not null;index"`
	DueAt       time.Time `gorm:"not null;index"`

	ClassroomID uint      `gorm:"not null;index"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	Submissions []AssignmentSubmission `gorm:"foreignKey:AssignmentID;references:ID"`
	Events      []Event                `gorm:"foreignKey:RelatedAssignmentID;references:ID"`
}
