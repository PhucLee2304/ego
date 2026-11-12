package dto

import (
	"time"

	"ego/services/knowledge/internal/model"
)

// ─── Request bodies ───────────────────────────────────────────────────────────

// CreateDeckBody là body tạo bộ thẻ mới.
type CreateDeckBody struct {
	Name        string  `json:"name"        validate:"required,notblank"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

// UpdateDeckBody là body cập nhật bộ thẻ (PATCH, mọi field đều tuỳ chọn).
type UpdateDeckBody struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Color       *string `json:"color"`
}

// SaveWordToDecksBody lưu một từ vào một hoặc nhiều bộ thẻ.
type SaveWordToDecksBody struct {
	WordID      uint    `json:"wordId"      validate:"required"`
	DeckIDs     []uint  `json:"deckIds"`
	NewDeckName *string `json:"newDeckName"`
	Note        *string `json:"note"`
}

// ReviewCardBody là body gửi kết quả ôn tập một thẻ.
type ReviewCardBody struct {
	Rating       string `json:"rating"       validate:"required,oneof=again hard good easy"`
	ReviewTimeMs int    `json:"reviewTimeMs"`
}

// UpdateFlashcardNoteBody là body cập nhật ghi chú của thẻ.
type UpdateFlashcardNoteBody struct {
	Note *string `json:"note"`
}

// ─── Response bodies ──────────────────────────────────────────────────────────

// DeckResponse là thông tin một bộ thẻ trả về client.
type DeckResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Color       *string   `json:"color"`
	CardCount   int       `json:"cardCount"`
	DueCount    int       `json:"dueCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// WordSaveStatusResponse trả về trạng thái lưu của một từ.
type WordSaveStatusResponse struct {
	WordID         uint   `json:"wordId"`
	IsSaved        bool   `json:"isSaved"`
	SavedInDeckIDs []uint `json:"savedInDeckIds"`
}

// ReviewCardResponse trả về trạng thái thẻ sau khi ôn tập.
type ReviewCardResponse struct {
	CardID       uint      `json:"cardId"`
	NextReviewAt time.Time `json:"nextReviewAt"`
	IntervalDays int       `json:"intervalDays"`
	Repetitions  int       `json:"repetitions"`
	Status       string    `json:"status"`
}

// Flashcard là thông tin một thẻ từ vựng trả về client.
type Flashcard struct {
	ID             uint       `json:"id"`
	DeckID         uint       `json:"deckId"`
	WordID         uint       `json:"wordId"`
	Note           *string    `json:"note"`
	Status         string     `json:"status"`
	IntervalDays   int        `json:"intervalDays"`
	EaseFactor     float64    `json:"easeFactor"`
	Repetitions    int        `json:"repetitions"`
	NextReviewAt   time.Time  `json:"nextReviewAt"`
	LastReviewedAt *time.Time `json:"lastReviewedAt"`
	Word           *Word      `json:"word"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// FlashcardStudyItem là thẻ được lấy ra để ôn tập (alias Flashcard).
type FlashcardStudyItem = Flashcard

// FlashcardUserStats là thống kê tổng quan thẻ của người dùng.
type FlashcardUserStats struct {
	TotalCards   int `json:"totalCards"`
	MasteredCards int `json:"masteredCards"`
	DueToday     int `json:"dueToday"`
	Streak       int `json:"streak"`
}

// ─── Mappers ──────────────────────────────────────────────────────────────────

// ToDeck chuyển model.FlashcardDeck sang DeckResponse.
func ToDeck(deck *model.FlashcardDeck, dueCount int) *DeckResponse {
	return &DeckResponse{
		ID:          deck.ID,
		Name:        deck.Name,
		Description: deck.Description,
		Color:       deck.Color,
		CardCount:   deck.CardCount,
		DueCount:    dueCount,
		CreatedAt:   deck.CreatedAt,
	}
}

// ToFlashcard chuyển model.Flashcard sang Flashcard DTO.
func ToFlashcard(fc *model.Flashcard) *Flashcard {
	var w *Word
	if fc.Word.ID != 0 {
		w = ToWord(&fc.Word)
	}
	return &Flashcard{
		ID:             fc.ID,
		DeckID:         fc.DeckID,
		WordID:         fc.WordID,
		Note:           fc.Note,
		Status:         string(fc.Status),
		IntervalDays:   fc.IntervalDays,
		EaseFactor:     fc.EaseFactor,
		Repetitions:    fc.Repetitions,
		NextReviewAt:   fc.NextReviewAt,
		LastReviewedAt: fc.LastReviewedAt,
		Word:           w,
		CreatedAt:      fc.CreatedAt,
	}
}

// ToReviewResponse chuyển model.Flashcard sang ReviewCardResponse sau khi ôn tập.
func ToReviewResponse(fc *model.Flashcard) *ReviewCardResponse {
	return &ReviewCardResponse{
		CardID:       fc.ID,
		NextReviewAt: fc.NextReviewAt,
		IntervalDays: fc.IntervalDays,
		Repetitions:  fc.Repetitions,
		Status:       string(fc.Status),
	}
}
