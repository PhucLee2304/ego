package model

import (
	"time"

	"gorm.io/gorm"
)

// FlashcardDeck là một nhóm / bộ thẻ từ vựng do người dùng tạo.
type FlashcardDeck struct {
	gorm.Model
	UserID      string  `gorm:"type:text;not null;index"`
	Name        string  `gorm:"type:text;not null"`
	Description *string `gorm:"type:text"`
	Color       *string `gorm:"type:text"`
	CardCount   int     `gorm:"not null;default:0"`

	Cards []Flashcard `gorm:"foreignKey:DeckID;constraint:OnDelete:CASCADE"`
}

// FlashcardStatus biểu diễn giai đoạn ghi nhớ của thẻ trong thuật toán SRS.
type FlashcardStatus string

const (
	FlashcardStatusNew      FlashcardStatus = "new"
	FlashcardStatusLearning FlashcardStatus = "learning"
	FlashcardStatusReview   FlashcardStatus = "review"
	FlashcardStatusMastered FlashcardStatus = "mastered"
)

// ReviewRating là đánh giá của người dùng sau khi lật thẻ (SM-2).
type ReviewRating string

const (
	ReviewRatingAgain ReviewRating = "again"
	ReviewRatingHard  ReviewRating = "hard"
	ReviewRatingGood  ReviewRating = "good"
	ReviewRatingEasy  ReviewRating = "easy"
)

// Flashcard là một từ vựng nằm trong một Deck cụ thể, kèm ghi chú riêng
// và trạng thái Spaced Repetition (SM-2). Mỗi cặp (DeckID, WordID) là duy nhất
// (trừ dòng đã xóa mềm).
type Flashcard struct {
	gorm.Model
	UserID string  `gorm:"type:text;not null;index"`
	DeckID uint    `gorm:"not null;default:0;index"`
	WordID uint    `gorm:"not null;index"`
	Note   *string `gorm:"type:text"`

	// SRS fields (SM-2)
	Status         FlashcardStatus `gorm:"type:text;not null;default:'new'"`
	IntervalDays   int             `gorm:"not null;default:0"`
	EaseFactor     float64         `gorm:"not null;default:2.5"`
	Repetitions    int             `gorm:"not null;default:0"`
	NextReviewAt   time.Time       `gorm:"not null;default:CURRENT_TIMESTAMP;index"`
	LastReviewedAt *time.Time

	Deck FlashcardDeck `gorm:"foreignKey:DeckID;references:ID"`
	Word Word          `gorm:"foreignKey:WordID;references:ID;constraint:OnDelete:CASCADE"`
}

// FlashcardReviewLog lưu lịch sử mỗi lần người dùng ôn tập thẻ.
type FlashcardReviewLog struct {
	gorm.Model
	UserID       string       `gorm:"type:text;not null;index"`
	FlashcardID  uint         `gorm:"not null;index"`
	DeckID       uint         `gorm:"not null;index"`
	Rating       ReviewRating `gorm:"type:text;not null"`
	ReviewTimeMs int          `gorm:"not null;default:0"`

	Flashcard Flashcard `gorm:"foreignKey:FlashcardID;references:ID;constraint:OnDelete:CASCADE"`
}
