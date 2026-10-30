package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ExamAudit struct {
	gorm.Model
	ExamID       uint           `gorm:"not null;index"`
	ActorID      string         `gorm:"type:text;not null;index"`
	Reason       string         `gorm:"type:text;not null"`
	Before       datatypes.JSON `gorm:"type:jsonb;not null"`
	After        datatypes.JSON `gorm:"type:jsonb;not null"`
	ScoreChanges datatypes.JSON `gorm:"type:jsonb;not null"`
	// Changes là danh sách dòng mô tả thay đổi (tiếng Anh) để hiện trong Change history.
	Changes    datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'"`
	RegradedAt *time.Time     `gorm:"index"`
}

type ExamAuditScoreChange struct {
	AttemptID   uint     `json:"attemptId"`
	StudentID   string   `json:"studentId"`
	BeforeScore *float64 `json:"beforeScore"`
	AfterScore  *float64 `json:"afterScore"`
}
