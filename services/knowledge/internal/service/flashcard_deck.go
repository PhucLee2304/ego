package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

// ─── Deck methods ─────────────────────────────────────────────────────────────

func (s *service) CreateDeck(ctx context.Context, userID string, body dto.CreateDeckBody) (*dto.DeckResponse, *httpx.HTTPError) {
	deck := &model.FlashcardDeck{
		UserID:      userID,
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	}
	if err := s.repo.CreateDeck(ctx, deck); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, httpx.NewError(http.StatusConflict, "[CONFLICT] Deck already exists", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to create deck", err)
	}
	return dto.ToDeck(deck, 0), nil
}

func (s *service) GetDecks(ctx context.Context, userID string) ([]*dto.DeckResponse, *httpx.HTTPError) {
	decks, err := s.repo.GetDecksByUser(ctx, userID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get decks", err)
	}
	if len(decks) == 0 {
		return []*dto.DeckResponse{}, nil
	}

	deckIDs := make([]uint, len(decks))
	for i, d := range decks {
		deckIDs[i] = d.ID
	}

	dueCounts, err := s.repo.CountDueCardsByDecks(ctx, userID, deckIDs, time.Now())
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to count due cards", err)
	}

	result := make([]*dto.DeckResponse, len(decks))
	for i, d := range decks {
		result[i] = dto.ToDeck(d, dueCounts[d.ID])
	}
	return result, nil
}

func (s *service) GetDeckByID(ctx context.Context, userID string, id uint) (*dto.DeckResponse, *httpx.HTTPError) {
	deck, err := s.repo.GetDeckByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Deck not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get deck", err)
	}

	dueCounts, err := s.repo.CountDueCardsByDecks(ctx, userID, []uint{deck.ID}, time.Now())
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to count due cards", err)
	}
	return dto.ToDeck(deck, dueCounts[deck.ID]), nil
}

func (s *service) UpdateDeck(ctx context.Context, userID string, id uint, body dto.UpdateDeckBody) (*dto.DeckResponse, *httpx.HTTPError) {
	deck, err := s.repo.GetDeckByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Deck not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get deck", err)
	}

	name := deck.Name
	if body.Name != nil {
		if trimmed := strings.TrimSpace(*body.Name); trimmed != "" {
			name = trimmed
		}
	}
	description := deck.Description
	if body.Description != nil {
		description = body.Description
	}
	color := deck.Color
	if body.Color != nil {
		color = body.Color
	}

	if err := s.repo.UpdateDeck(ctx, userID, id, name, description, color); err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to update deck", err)
	}
	deck.Name = name
	deck.Description = description
	deck.Color = color

	dueCounts, err := s.repo.CountDueCardsByDecks(ctx, userID, []uint{deck.ID}, time.Now())
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to count due cards", err)
	}
	return dto.ToDeck(deck, dueCounts[deck.ID]), nil
}

func (s *service) DeleteDeck(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	if err := s.repo.DeleteDeck(ctx, userID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Deck not found", err)
		}
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete deck", err)
	}
	return nil
}
