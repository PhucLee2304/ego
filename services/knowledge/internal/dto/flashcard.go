package dto

import (
	"time"

	"ego/services/knowledge/internal/model"
)

type CreateFlashcardBody struct {
	WordID uint    `json:"wordId" validate:"required"`
	Note   *string `json:"note"`
}

type Flashcard struct {
	ID        uint      `json:"id"`
	Note      *string   `json:"note"`
	CreatedAt time.Time `json:"createdAt"`
	Word      *Word     `json:"word"`
}

func ToFlashcard(flashcard *model.Flashcard) *Flashcard {
	return &Flashcard{
		ID:        flashcard.ID,
		Note:      flashcard.Note,
		CreatedAt: flashcard.CreatedAt,
		Word:      ToWord(&flashcard.Word),
	}
}
