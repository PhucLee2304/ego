package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

func (s *service) GetFlashcards(ctx context.Context, userID string, query httpx.PaginationQuery) ([]*dto.Flashcard, int64, *httpx.HTTPError) {
	flashcards, total, err := s.repo.GetFlashcards(ctx, userID, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get flashcards", err)
	}

	flashcardDTOs := make([]*dto.Flashcard, len(flashcards))
	for i, flashcard := range flashcards {
		flashcardDTOs[i] = dto.ToFlashcard(flashcard)
	}
	return flashcardDTOs, total, nil
}

func (s *service) CreateFlashcard(ctx context.Context, userID string, body dto.CreateFlashcardBody) (*dto.Flashcard, *httpx.HTTPError) {
	flashcard := &model.Flashcard{UserID: userID, WordID: body.WordID}
	// Ghi chú rỗng hoặc chỉ có khoảng trắng thì lưu NULL.
	if body.Note != nil {
		if note := strings.TrimSpace(*body.Note); note != "" {
			flashcard.Note = &note
		}
	}

	if err := s.repo.CreateFlashcard(ctx, flashcard); err != nil {
		if errors.Is(err, gorm.ErrForeignKeyViolated) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Word not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to create flashcard", err)
	}
	return dto.ToFlashcard(flashcard), nil
}

func (s *service) DeleteFlashcard(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	if err := s.repo.DeleteFlashcard(ctx, userID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Flashcard not found", err)
		}
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete flashcard", err)
	}
	return nil
}
