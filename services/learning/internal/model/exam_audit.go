package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ExamAudit là lịch sử chỉnh sửa đề private của bài tập lớp, hiện ở mục Change history.
// Một bản ghi được tạo mỗi lần giáo viên sửa nội dung đề, đính chính đáp án (chấm lại các
// bài đã nộp) hoặc đổi lịch/cài đặt của bài tập.
//   - ActorID, Reason: giáo viên thực hiện và lý do nhập khi sửa.
//   - Before, After: bản chụp JSON của đề trước và sau khi sửa; "{}" nếu chỉ đổi lịch/cài đặt.
//   - ScoreChanges: danh sách ExamAuditScoreChange của các bài bị đổi điểm; "[]" nếu không chấm lại.
//   - Changes: danh sách dòng mô tả thay đổi (tiếng Anh) để hiện trong Change history.
//   - RegradedAt: lúc chấm lại, chỉ có khi đính chính đáp án.
type ExamAudit struct {
	gorm.Model
	ExamID       uint           `gorm:"not null;index"`
	ActorID      string         `gorm:"type:text;not null;index"`
	Reason       string         `gorm:"type:text;not null"`
	Before       datatypes.JSON `gorm:"type:jsonb;not null"`
	After        datatypes.JSON `gorm:"type:jsonb;not null"`
	ScoreChanges datatypes.JSON `gorm:"type:jsonb;not null"`
	Changes      datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'"`
	RegradedAt   *time.Time     `gorm:"index"`
}

// ExamAuditScoreChange là điểm của một bài nộp trước và sau khi chấm lại, lưu trong ExamAudit.ScoreChanges.
type ExamAuditScoreChange struct {
	AttemptID   uint     `json:"attemptId"`
	StudentID   string   `json:"studentId"`
	BeforeScore *float64 `json:"beforeScore"`
	AfterScore  *float64 `json:"afterScore"`
}
