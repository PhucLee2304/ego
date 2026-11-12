package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
)

type fakeFlashcardRepository struct {
	repository.Repository
	card       *model.Flashcard
	cardErr    error
	updatedSRS *model.Flashcard
	reviewLog  *model.FlashcardReviewLog

	deck    *model.FlashcardDeck
	deckErr error

	gotUserID string
	gotID     uint
	err       error
}

func (f *fakeFlashcardRepository) GetFlashcardByID(_ context.Context, userID string, id uint) (*model.Flashcard, error) {
	f.gotUserID, f.gotID = userID, id
	return f.card, f.cardErr
}

func (f *fakeFlashcardRepository) UpdateFlashcardSRS(_ context.Context, fc *model.Flashcard) error {
	f.updatedSRS = fc
	return f.err
}

func (f *fakeFlashcardRepository) CreateReviewLog(_ context.Context, log *model.FlashcardReviewLog) error {
	f.reviewLog = log
	return f.err
}

func (f *fakeFlashcardRepository) CreateDeck(_ context.Context, deck *model.FlashcardDeck) error {
	f.deck = deck
	deck.ID = 10
	deck.CreatedAt = time.Now()
	return f.deckErr
}

func (f *fakeFlashcardRepository) DeleteDeck(_ context.Context, userID string, id uint) error {
	f.gotUserID, f.gotID = userID, id
	return f.err
}

func (f *fakeFlashcardRepository) DeleteFlashcard(_ context.Context, userID string, id uint) error {
	f.gotUserID, f.gotID = userID, id
	return f.err
}

func TestReviewFlashcard_SM2(t *testing.T) {
	tests := []struct {
		name          string
		initialCard   *model.Flashcard
		rating        string
		wantStatus    model.FlashcardStatus
		wantInterval  int
		wantReps      int
		wantEFMin     float64
		wantEFMax     float64
		wantHTTPCode  int
	}{
		{
			name: "again resets repetitions and sets interval to 1 day",
			initialCard: &model.Flashcard{
				UserID:       "u1",
				DeckID:       1,
				WordID:       2,
				Status:       model.FlashcardStatusReview,
				IntervalDays: 10,
				EaseFactor:   2.5,
				Repetitions:  3,
			},
			rating:       "again",
			wantStatus:   model.FlashcardStatusLearning,
			wantInterval: 1,
			wantReps:     0,
			wantEFMin:    2.29,
			wantEFMax:    2.31,
		},
		{
			name: "hard increases interval slightly and drops ease factor",
			initialCard: &model.Flashcard{
				UserID:       "u1",
				DeckID:       1,
				WordID:       2,
				Status:       model.FlashcardStatusReview,
				IntervalDays: 10,
				EaseFactor:   2.5,
				Repetitions:  2,
			},
			rating:       "hard",
			wantStatus:   model.FlashcardStatusLearning,
			wantInterval: 12,
			wantReps:     2,
			wantEFMin:    2.34,
			wantEFMax:    2.36,
		},
		{
			name: "good first repetition sets interval to 1 day",
			initialCard: &model.Flashcard{
				UserID:       "u1",
				DeckID:       1,
				WordID:       2,
				Status:       model.FlashcardStatusNew,
				IntervalDays: 0,
				EaseFactor:   2.5,
				Repetitions:  0,
			},
			rating:       "good",
			wantStatus:   model.FlashcardStatusReview,
			wantInterval: 1,
			wantReps:     1,
			wantEFMin:    2.49,
			wantEFMax:    2.51,
		},
		{
			name: "good second repetition sets interval to 6 days",
			initialCard: &model.Flashcard{
				UserID:       "u1",
				DeckID:       1,
				WordID:       2,
				Status:       model.FlashcardStatusReview,
				IntervalDays: 1,
				EaseFactor:   2.5,
				Repetitions:  1,
			},
			rating:       "good",
			wantStatus:   model.FlashcardStatusReview,
			wantInterval: 6,
			wantReps:     2,
			wantEFMin:    2.49,
			wantEFMax:    2.51,
		},
		{
			name: "easy repetition reaches mastered after 5 reps",
			initialCard: &model.Flashcard{
				UserID:       "u1",
				DeckID:       1,
				WordID:       2,
				Status:       model.FlashcardStatusReview,
				IntervalDays: 15,
				EaseFactor:   2.5,
				Repetitions:  4,
			},
			rating:       "easy",
			wantStatus:   model.FlashcardStatusMastered,
			wantInterval: 52, // 15 * (2.5 + 0.15) * 1.3 = 15 * 2.65 * 1.3 = 51.675 -> 52
			wantReps:     5,
			wantEFMin:    2.64,
			wantEFMax:    2.66,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initialCard.ID = 100
			repo := &fakeFlashcardRepository{card: tt.initialCard}
			svc := New(repo)

			resp, httpErr := svc.ReviewFlashcard(context.Background(), "u1", 100, dto.ReviewCardBody{
				Rating:       tt.rating,
				ReviewTimeMs: 1500,
			})

			if tt.wantHTTPCode != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantHTTPCode {
					t.Fatalf("want status %d, got %v", tt.wantHTTPCode, httpErr)
				}
				return
			}

			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}

			if resp.CardID != 100 {
				t.Errorf("CardID = %d, want 100", resp.CardID)
			}
			if resp.Status != string(tt.wantStatus) {
				t.Errorf("Status = %s, want %s", resp.Status, tt.wantStatus)
			}
			if resp.IntervalDays != tt.wantInterval {
				t.Errorf("IntervalDays = %d, want %d", resp.IntervalDays, tt.wantInterval)
			}
			if resp.Repetitions != tt.wantReps {
				t.Errorf("Repetitions = %d, want %d", resp.Repetitions, tt.wantReps)
			}
			if repo.updatedSRS.EaseFactor < tt.wantEFMin || repo.updatedSRS.EaseFactor > tt.wantEFMax {
				t.Errorf("EaseFactor = %f, want between %f and %f", repo.updatedSRS.EaseFactor, tt.wantEFMin, tt.wantEFMax)
			}
			if repo.reviewLog == nil || repo.reviewLog.Rating != model.ReviewRating(tt.rating) {
				t.Errorf("ReviewLog not created properly: %#v", repo.reviewLog)
			}
		})
	}
}

func TestCreateDeck(t *testing.T) {
	repo := &fakeFlashcardRepository{}
	svc := New(repo)

	resp, httpErr := svc.CreateDeck(context.Background(), "u1", dto.CreateDeckBody{
		Name: "IELTS Vocab",
	})
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if resp.ID != 10 || resp.Name != "IELTS Vocab" {
		t.Fatalf("unexpected deck response: %#v", resp)
	}

	repoErr := &fakeFlashcardRepository{deckErr: gorm.ErrDuplicatedKey}
	_, httpErr = New(repoErr).CreateDeck(context.Background(), "u1", dto.CreateDeckBody{Name: "IELTS Vocab"})
	if httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("want 409 conflict, got %v", httpErr)
	}
}

func TestDeleteDeck(t *testing.T) {
	repo := &fakeFlashcardRepository{}
	svc := New(repo)

	httpErr := svc.DeleteDeck(context.Background(), "u1", 5)
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotUserID != "u1" || repo.gotID != 5 {
		t.Fatalf("bad repo args: %q, %d", repo.gotUserID, repo.gotID)
	}

	repoErr := &fakeFlashcardRepository{err: gorm.ErrRecordNotFound}
	httpErr = New(repoErr).DeleteDeck(context.Background(), "u1", 5)
	if httpErr == nil || httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %v", httpErr)
	}
}
