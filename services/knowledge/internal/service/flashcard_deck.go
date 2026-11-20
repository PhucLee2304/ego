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

func (s *service) CreateDeck(ctx context.Context, userID string, body dto.CreateDeckBody) (*dto.DeckResponse, *httpx.HTTPError) {
	deck := &model.FlashcardDeck{
		UserID:      userID,
		Name:        strings.TrimSpace(body.Name),
		Description: body.Description,
		Color:       body.Color,
	}
	if err := s.repo.CreateDeck(ctx, deck); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, httpx.NewError(http.StatusConflict, "[CONFLICT] Deck already exists", err)
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to create deck")
	}
	return dto.ToDeck(deck, 0), nil
}

func (s *service) GetDecks(ctx context.Context, userID string) ([]*dto.DeckResponse, *httpx.HTTPError) {
	decks, err := s.repo.GetDecksByUser(ctx, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get decks")
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
		return nil, httpx.FromError(err, "[ERROR] Failed to count due cards")
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
		return nil, httpx.FromError(err, "[ERROR] Failed to get deck")
	}

	dueCounts, err := s.repo.CountDueCardsByDecks(ctx, userID, []uint{deck.ID}, time.Now())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to count due cards")
	}
	return dto.ToDeck(deck, dueCounts[deck.ID]), nil
}

func (s *service) UpdateDeck(ctx context.Context, userID string, id uint, body dto.UpdateDeckBody) (*dto.DeckResponse, *httpx.HTTPError) {
	deck, err := s.repo.GetDeckByID(ctx, userID, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get deck")
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
		return nil, httpx.FromError(err, "[ERROR] Failed to update deck")
	}
	deck.Name = name
	deck.Description = description
	deck.Color = color

	dueCounts, err := s.repo.CountDueCardsByDecks(ctx, userID, []uint{deck.ID}, time.Now())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to count due cards")
	}
	return dto.ToDeck(deck, dueCounts[deck.ID]), nil
}

func (s *service) DeleteDeck(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	if err := s.repo.DeleteDeck(ctx, userID, id); err != nil {
		return httpx.FromError(err, "[ERROR] Failed to delete deck")
	}
	return nil
}
