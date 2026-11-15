package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type History struct {
	gorm.Model
	// Data là bản chụp JSON đầy đủ của bài đã nộp (dto.AttemptResponse: câu hỏi, đáp án đã chọn,
	// đáp án đúng, giải thích, điểm), tạo bằng dto.MarshalAttemptHistory. Màn xem lại bài đọc
	// thẳng từ đây, không dựng lại từ đề, nên sửa đề về sau không làm đổi bài cũ. Được ghi lại
	// khi điểm thay đổi (đính chính đáp án, đình chỉ, khôi phục).
	Data datatypes.JSON `gorm:"type:jsonb;not null"`

	AttemptID uint    `gorm:"not null;uniqueIndex"`
	Attempt   Attempt `gorm:"foreignKey:AttemptID;references:ID"`

	ExamTitle string   `gorm:"not null"`
	ExamType  ExamType `gorm:"type:text;not null;index"`
	ExamYear  *int
}
