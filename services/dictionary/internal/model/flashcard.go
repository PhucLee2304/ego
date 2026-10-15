package model

import "gorm.io/gorm"

// UserFlashcard represents a word that a user has saved as a flashcard.
type UserFlashcard struct {
	gorm.Model
	UserID string `gorm:"not null;index"`
	WordID uint   `gorm:"not null;index"`
	Note   string `gorm:"type:text;default:''"`

	Word Word `gorm:"foreignKey:WordID;constraint:OnDelete:CASCADE"`
}
