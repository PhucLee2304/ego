package service

import (
	"context"
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"

	"gorm.io/gorm"
)

func (s *service) GetThemes(ctx context.Context) ([]*dto.Theme, *httpx.HTTPError) {
	themes, err := s.repo.GetThemes(ctx)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get themes", err)
	}

	themeDTOs := make([]*dto.Theme, len(themes))
	for i, theme := range themes {
		themeDTOs[i] = dto.ToTheme(theme)
	}
	return themeDTOs, nil
}

func (s *service) GetWordsByTheme(ctx context.Context, id uint, query httpx.PaginationQuery) (*dto.GetThemeWordsResponse, *httpx.HTTPError) {
	theme, words, total, err := s.repo.GetThemeByID(ctx, id, query.Limit(), query.Offset())
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Theme not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get theme", err)
	}

	wordDTOs := make([]*dto.ThemeWord, len(words))
	for i, word := range words {
		wordDTOs[i] = dto.ToThemeWord(word)
	}
	return &dto.GetThemeWordsResponse{
		PaginatedResponse: httpx.ToPaginatedResponse(wordDTOs, query, total),
		Topic:             dto.ToTheme(theme),
	}, nil
}
