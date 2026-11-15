package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

func (s *service) SaveWordToDecks(ctx context.Context, userID string, body dto.SaveWordToDecksBody) *httpx.HTTPError {
	newDeckName := ""
	if body.NewDeckName != nil {
		newDeckName = strings.TrimSpace(*body.NewDeckName)
	}
	err := s.repo.SaveWordToDecks(ctx, userID, body.WordID, body.DeckIDs, newDeckName, body.Note)
	return httpx.FromError(err, "[ERROR] Failed to save word to decks")
}

func (s *service) GetWordSaveStatus(ctx context.Context, userID string, wordID uint) (*dto.WordSaveStatusResponse, *httpx.HTTPError) {
	deckIDs, err := s.repo.GetWordSaveStatus(ctx, userID, wordID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get word save status")
	}
	return &dto.WordSaveStatusResponse{
		WordID:         wordID,
		IsSaved:        len(deckIDs) > 0,
		SavedInDeckIDs: deckIDs,
	}, nil
}

func (s *service) GetCardsByDeck(ctx context.Context, userID string, deckID uint, query httpx.PaginationQuery) ([]*dto.Flashcard, int64, *httpx.HTTPError) {
	cards, total, err := s.repo.GetCardsByDeck(ctx, userID, deckID, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.FromError(err, "[ERROR] Failed to get cards")
	}
	result := make([]*dto.Flashcard, len(cards))
	for i, c := range cards {
		result[i] = dto.ToFlashcard(c)
	}
	return result, total, nil
}

func (s *service) UpdateFlashcardNote(ctx context.Context, userID string, id uint, note *string) (*dto.Flashcard, *httpx.HTTPError) {
	card, err := s.repo.UpdateFlashcardNote(ctx, userID, id, note)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update flashcard note")
	}
	return dto.ToFlashcard(card), nil
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

func (s *service) GetStudyCards(ctx context.Context, userID string, deckID uint) ([]*dto.Flashcard, *httpx.HTTPError) {
	cards, err := s.repo.GetStudyCards(ctx, userID, deckID, 20, time.Now())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get study cards")
	}
	result := make([]*dto.Flashcard, len(cards))
	for i, c := range cards {
		result[i] = dto.ToFlashcard(c)
	}
	return result, nil
}

func (s *service) GetAllDueCards(ctx context.Context, userID string) ([]*dto.Flashcard, *httpx.HTTPError) {
	cards, err := s.repo.GetAllDueCards(ctx, userID, 50, time.Now())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get due cards")
	}
	result := make([]*dto.Flashcard, len(cards))
	for i, c := range cards {
		result[i] = dto.ToFlashcard(c)
	}
	return result, nil
}

func (s *service) ReviewFlashcard(ctx context.Context, userID string, id uint, body dto.ReviewCardBody) (*dto.ReviewCardResponse, *httpx.HTTPError) {
	card, err := s.repo.GetFlashcardByID(ctx, userID, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get flashcard")
	}

	rating := model.ReviewRating(body.Rating)
	applySM2(card, rating)
	now := time.Now()
	card.LastReviewedAt = &now
	card.NextReviewAt = now.AddDate(0, 0, card.IntervalDays)

	err = s.repo.ReviewFlashcard(ctx, card, &model.FlashcardReviewLog{
		UserID:       userID,
		FlashcardID:  card.ID,
		DeckID:       card.DeckID,
		Rating:       rating,
		ReviewTimeMs: body.ReviewTimeMs,
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to save review")
	}
	return dto.ToReviewResponse(card), nil
}

func (s *service) GetFlashcardUserStats(ctx context.Context, userID string) (*dto.FlashcardUserStats, *httpx.HTTPError) {
	stats, err := s.repo.GetFlashcardStats(ctx, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get flashcard stats")
	}
	return stats, nil
}

// ─── SM-2 algorithm ───────────────────────────────────────────────────────────

// applySM2 cập nhật trường SRS của card theo thuật toán SM-2 dựa trên rating.
func applySM2(card *model.Flashcard, rating model.ReviewRating) {
	switch rating {
	case model.ReviewRatingAgain:
		card.Repetitions = 0
		card.IntervalDays = 1
		card.Status = model.FlashcardStatusLearning
		card.EaseFactor = math.Max(1.3, card.EaseFactor-0.20)

	case model.ReviewRatingHard:
		if card.Repetitions < 1 {
			card.Repetitions = 1
		}
		interval := int(float64(card.IntervalDays) * 1.2)
		if interval < 1 {
			interval = 1
		}
		card.IntervalDays = interval
		card.EaseFactor = math.Max(1.3, card.EaseFactor-0.15)
		card.Status = model.FlashcardStatusLearning

	case model.ReviewRatingGood:
		card.Repetitions++
		switch card.Repetitions {
		case 1:
			card.IntervalDays = 1
		case 2:
			card.IntervalDays = 6
		default:
			card.IntervalDays = int(math.Round(float64(card.IntervalDays) * card.EaseFactor))
		}
		card.Status = model.FlashcardStatusReview

	case model.ReviewRatingEasy:
		card.Repetitions++
		card.EaseFactor += 0.15
		if card.Repetitions <= 1 {
			card.IntervalDays = 4
		} else {
			card.IntervalDays = int(math.Round(float64(card.IntervalDays) * card.EaseFactor * 1.3))
		}
		if card.Repetitions >= 5 {
			card.Status = model.FlashcardStatusMastered
		} else {
			card.Status = model.FlashcardStatusReview
		}
	}
}
