package model

import "gorm.io/gorm"

type Question struct {
	gorm.Model
	Content string `gorm:"type:text;not null"`
	// Part TOEIC của câu (1–7), chi tiết hơn Section.Code (LISTENING/READING/FULL):
	// part 1–4 thuộc LISTENING, 5–7 thuộc READING. nil với đề không chia part (ASSIGNMENT).
	Part        *PartCode `gorm:"type:text;index"`
	Explanation string    `gorm:"type:text"`
	// Thứ tự câu trong group (nếu GroupID có) hoặc trong section, dùng để sắp xếp.
	Order int `gorm:"not null"`
	// Số thứ tự của câu này trong đề gốc (TOEIC: 1–200), hiển thị là "Câu N".
	// Khác Order (thứ tự trong section/group). nil thì đánh số theo thứ tự toàn bài.
	Number            *int
	ExcludedFromScore bool `gorm:"not null;default:false;index"`

	SectionID uint    `gorm:"not null;index"`
	Section   Section `gorm:"foreignKey:SectionID;references:ID"`
	GroupID   *uint   `gorm:"index"`
	Group     *Group  `gorm:"foreignKey:GroupID;references:ID"`

	Options []Option `gorm:"foreignKey:QuestionID;references:ID"`
}

type PartCode string

const (
	Part1 PartCode = "1"
	Part2 PartCode = "2"
	Part3 PartCode = "3"
	Part4 PartCode = "4"
	Part5 PartCode = "5"
	Part6 PartCode = "6"
	Part7 PartCode = "7"
)
