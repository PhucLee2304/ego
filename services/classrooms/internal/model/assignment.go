package model

import (
	"time"

	"gorm.io/gorm"
)

type Assignment struct {
	gorm.Model
	Title       string    `gorm:"type:text;not null"`
	Description *string   `gorm:"type:text"`
	ExamID      uint      `gorm:"not null;index;uniqueIndex:idx_assignments_classroom_exam"`
	Duration    int       `gorm:"not null;default:60"`
	OpensAt     time.Time `gorm:"not null;index"`
	DueAt       time.Time `gorm:"not null;index"`
	Active      bool      `gorm:"not null;default:true;index"`

	ClassroomID uint      `gorm:"not null;index;uniqueIndex:idx_assignments_classroom_exam"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	Submissions []AssignmentSubmission `gorm:"foreignKey:AssignmentID;references:ID"`
}
