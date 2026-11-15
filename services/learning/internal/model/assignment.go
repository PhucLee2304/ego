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

	// Giám sát làm bài:
	//   - Proctored: bật thì học viên phải đồng ý điều khoản giám sát trước khi làm, và các lần
	//     rời màn hình/chuyển tab bị ghi nhận là vi phạm.
	//   - MaxViolations: số lần vi phạm được phép (≥ 1, mặc định 2); vượt quá thì bài bị đình
	//     chỉ và 0 điểm. Giá trị được chụp vào attempt lúc bắt đầu, sửa sau không ảnh hưởng
	//     bài đang làm.
	Proctored     bool `gorm:"not null;default:false"`
	MaxViolations int  `gorm:"not null;default:2"`

	ClassroomID uint      `gorm:"not null;index;uniqueIndex:idx_assignments_classroom_exam"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	Submissions []AssignmentSubmission `gorm:"foreignKey:AssignmentID;references:ID"`
}
