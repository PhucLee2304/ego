package service

import (
	"context"
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"

	"gorm.io/gorm"
)

func (s *service) GetWords(ctx context.Context, query dto.GetWordsQuery) ([]*dto.Word, int64, *httpx.HTTPError) {
	words, total, err := s.repo.GetWords(ctx, query.Query, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get words", err)
	}

	wordDTOs := make([]*dto.Word, len(words))
	for i, word := range words {
		wordDTOs[i] = dto.ToWord(word)
	}
	return wordDTOs, total, nil
}

func (s *service) GetWordByID(ctx context.Context, id uint) (*dto.Word, *httpx.HTTPError) {
	word, err := s.repo.GetWordByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Word not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get word", err)
	}
	return dto.ToWord(word), nil
}
