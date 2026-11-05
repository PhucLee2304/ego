package model

import "gorm.io/gorm"

// Flashcard là một từ người dùng lưu lại để ôn, kèm ghi chú riêng.
// Một người có thể lưu cùng một từ nhiều lần (giữ hành vi cũ).
type Flashcard struct {
	gorm.Model
	UserID string  `gorm:"not null;index"`
	Note   *string `gorm:"type:text"`

	WordID uint `gorm:"not null;index"`
	Word   Word `gorm:"foreignKey:WordID;references:ID;constraint:OnDelete:CASCADE"`
}
