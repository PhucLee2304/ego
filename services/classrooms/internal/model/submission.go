package model

import (
	"time"

	"gorm.io/gorm"
)

type AssignmentSubmission struct {
	gorm.Model
	AssignmentID uint       `gorm:"not null;index;uniqueIndex:idx_assignment_submissions_assignment_student"`
	Assignment   Assignment `gorm:"foreignKey:AssignmentID;references:ID"`
	StudentID    string     `gorm:"type:text;not null;index;uniqueIndex:idx_assignment_submissions_assignment_student"`
	AttemptID    uint       `gorm:"not null;uniqueIndex"`

	Score *float64 `gorm:"type:numeric(5,2)"`
}

type SubmissionStatus string

const (
	SubmissionStatusInProgress SubmissionStatus = "IN_PROGRESS"
	SubmissionStatusSubmitted  SubmissionStatus = "SUBMITTED"
	SubmissionStatusLate       SubmissionStatus = "LATE"
)

func (s AssignmentSubmission) Status(now time.Time, dueAt time.Time) SubmissionStatus {
	if s.Score != nil {
		return SubmissionStatusSubmitted
	}
	if now.After(dueAt) {
		return SubmissionStatusLate
	}
	return SubmissionStatusInProgress
}
