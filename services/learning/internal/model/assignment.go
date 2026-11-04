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

	// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 1.1): MaxViolations là số lần
	// vi phạm được phép (≥ 1), vượt quá thì đình chỉ.
	Proctored     bool `gorm:"not null;default:false"`
	MaxViolations int  `gorm:"not null;default:2"`

	ClassroomID uint      `gorm:"not null;index;uniqueIndex:idx_assignments_classroom_exam"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	Submissions []AssignmentSubmission `gorm:"foreignKey:AssignmentID;references:ID"`
}
