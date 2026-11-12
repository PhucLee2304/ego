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
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
)

// ─── Card & Study methods ──────────────────────────────────────────────────────

func (s *service) SaveWordToDecks(ctx context.Context, userID string, body dto.SaveWordToDecksBody) *httpx.HTTPError {
	txErr := s.repo.Transaction(ctx, func(txRepo repository.Repository) error {
		return s.withRepo(txRepo).saveWordToDecks(ctx, userID, body)
	})
	return mapErr(txErr, "save word to decks")
}

// saveWordToDecks chạy bên trong transaction.
func (s *service) saveWordToDecks(ctx context.Context, userID string, body dto.SaveWordToDecksBody) error {
	if _, err := s.repo.GetWordByID(ctx, body.WordID); err != nil {
		return err
	}

	targetDeckIDs := make([]uint, len(body.DeckIDs))
	copy(targetDeckIDs, body.DeckIDs)

	if body.NewDeckName != nil && strings.TrimSpace(*body.NewDeckName) != "" {
		name := strings.TrimSpace(*body.NewDeckName)
		newDeck := &model.FlashcardDeck{UserID: userID, Name: name}
		if err := s.repo.CreateDeck(ctx, newDeck); err != nil {
			return err
		}
		targetDeckIDs = append(targetDeckIDs, newDeck.ID)
	}

	now := time.Now()
	for _, deckID := range targetDeckIDs {
		exists, err := s.repo.IsWordInDeck(ctx, userID, deckID, body.WordID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		card := &model.Flashcard{
			UserID:       userID,
			DeckID:       deckID,
			WordID:       body.WordID,
			Note:         body.Note,
			Status:       model.FlashcardStatusNew,
			EaseFactor:   2.5,
			NextReviewAt: now,
		}
		if err := s.repo.CreateFlashcard(ctx, card); err != nil {
			return err
		}
		if err := s.repo.UpdateDeckCardCount(ctx, deckID, 1); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) GetWordSaveStatus(ctx context.Context, userID string, wordID uint) (*dto.WordSaveStatusResponse, *httpx.HTTPError) {
	deckIDs, err := s.repo.GetWordSaveStatus(ctx, userID, wordID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get word save status", err)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, 0, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Deck not found", err)
		}
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get cards", err)
	}
	result := make([]*dto.Flashcard, len(cards))
	for i, c := range cards {
		result[i] = dto.ToFlashcard(c)
	}
	return result, total, nil
}

func (s *service) UpdateFlashcardNote(ctx context.Context, userID string, id uint, note *string) (*dto.Flashcard, *httpx.HTTPError) {
	card, err := s.repo.GetFlashcardByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Flashcard not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get flashcard", err)
	}
	card.Note = note
	if err := s.repo.UpdateFlashcardSRS(ctx, card); err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to update flashcard", err)
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
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get study cards", err)
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
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get due cards", err)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Flashcard not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get flashcard", err)
	}

	applySM2(card, model.ReviewRating(body.Rating))

	now := time.Now()
	card.LastReviewedAt = &now
	card.NextReviewAt = now.AddDate(0, 0, card.IntervalDays)

	if err := s.repo.UpdateFlashcardSRS(ctx, card); err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to update flashcard", err)
	}

	log := &model.FlashcardReviewLog{
		UserID:       userID,
		FlashcardID:  card.ID,
		DeckID:       card.DeckID,
		Rating:       model.ReviewRating(body.Rating),
		ReviewTimeMs: body.ReviewTimeMs,
	}
	if err := s.repo.CreateReviewLog(ctx, log); err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to save review log", err)
	}

	return dto.ToReviewResponse(card), nil
}

func (s *service) GetFlashcardUserStats(ctx context.Context, userID string) (*dto.FlashcardUserStats, *httpx.HTTPError) {
	stats, err := s.repo.GetFlashcardStats(ctx, userID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get flashcard stats", err)
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

// ─── Error helper ─────────────────────────────────────────────────────────────

func mapErr(err error, action string) *httpx.HTTPError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, gorm.ErrForeignKeyViolated):
		return httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Resource not found", err)
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return httpx.NewError(http.StatusConflict, "[CONFLICT] Resource already exists", err)
	default:
		return httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to "+action, err)
	}
}
