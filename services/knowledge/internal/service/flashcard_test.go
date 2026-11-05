package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
)

type fakeFlashcardRepository struct {
	repository.Repository
	flashcards []*model.Flashcard
	total      int64
	err        error

	gotUserID string
	gotLimit  int32
	gotOffset int32
	gotID     uint
	created   *model.Flashcard
}

func (f *fakeFlashcardRepository) GetFlashcards(_ context.Context, userID string, limit, offset int32) ([]*model.Flashcard, int64, error) {
	f.gotUserID, f.gotLimit, f.gotOffset = userID, limit, offset
	return f.flashcards, f.total, f.err
}

func (f *fakeFlashcardRepository) CreateFlashcard(_ context.Context, flashcard *model.Flashcard) error {
	f.created = flashcard
	if f.err != nil {
		return f.err
	}
	flashcard.ID = 9
	flashcard.Word = model.Word{Word: "glad"}
	flashcard.Word.ID = flashcard.WordID
	return nil
}

func (f *fakeFlashcardRepository) DeleteFlashcard(_ context.Context, userID string, id uint) error {
	f.gotUserID, f.gotID = userID, id
	return f.err
}

func TestGetFlashcards(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	flashcard := &model.Flashcard{UserID: "u1", Note: ptr("remember"), WordID: 7, Word: model.Word{Word: "glad", Phonetic: ptr("/g/")}}
	flashcard.ID, flashcard.CreatedAt, flashcard.Word.ID = 3, createdAt, 7

	repo := &fakeFlashcardRepository{flashcards: []*model.Flashcard{flashcard}, total: 11}
	got, total, httpErr := New(repo).GetFlashcards(context.Background(), "u1", httpx.PaginationQuery{Page: 2, PageSize: 5})
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotUserID != "u1" || repo.gotLimit != 5 || repo.gotOffset != 5 {
		t.Fatalf("bad repo args: %q %d %d", repo.gotUserID, repo.gotLimit, repo.gotOffset)
	}
	card := got[0]
	if total != 11 || card.ID != 3 || *card.Note != "remember" || !card.CreatedAt.Equal(createdAt) {
		t.Fatalf("bad flashcard mapping: %#v", card)
	}
	if card.Word.ID != 7 || card.Word.Word != "glad" || *card.Word.Phonetic != "/g/" {
		t.Fatalf("bad word mapping: %#v", card.Word)
	}

	empty, _, _ := New(&fakeFlashcardRepository{}).GetFlashcards(context.Background(), "u1", httpx.PaginationQuery{Page: 1, PageSize: 10})
	if empty == nil {
		t.Fatal("empty list must be a non-nil slice")
	}

	if _, _, httpErr := New(&fakeFlashcardRepository{err: errDB}).GetFlashcards(context.Background(), "u1", httpx.PaginationQuery{Page: 1, PageSize: 10}); httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", httpErr)
	}
}

func TestCreateFlashcard(t *testing.T) {
	tests := []struct {
		name       string
		body       dto.CreateFlashcardBody
		repo       *fakeFlashcardRepository
		wantStatus int
		wantNote   *string
	}{
		{name: "creates with trimmed note", body: dto.CreateFlashcardBody{WordID: 7, Note: ptr("  remember  ")}, repo: &fakeFlashcardRepository{}, wantNote: ptr("remember")},
		{name: "blank note is stored as null", body: dto.CreateFlashcardBody{WordID: 7, Note: ptr("   ")}, repo: &fakeFlashcardRepository{}},
		{name: "missing note is stored as null", body: dto.CreateFlashcardBody{WordID: 7}, repo: &fakeFlashcardRepository{}},
		{name: "unknown word is 404", body: dto.CreateFlashcardBody{WordID: 7}, repo: &fakeFlashcardRepository{err: gorm.ErrForeignKeyViolated}, wantStatus: http.StatusNotFound},
		{name: "db error is 500", body: dto.CreateFlashcardBody{WordID: 7}, repo: &fakeFlashcardRepository{err: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, httpErr := New(tt.repo).CreateFlashcard(context.Background(), "u1", tt.body)
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
				}
				return
			}
			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}
			created := tt.repo.created
			if created.UserID != "u1" || created.WordID != 7 {
				t.Fatalf("bad created flashcard: %#v", created)
			}
			if (tt.wantNote == nil) != (created.Note == nil) || (tt.wantNote != nil && *created.Note != *tt.wantNote) {
				t.Fatalf("want note %v, got %v", tt.wantNote, created.Note)
			}
			if got.ID != 9 || got.Word.ID != 7 || got.Word.Word != "glad" {
				t.Fatalf("bad response: %#v", got)
			}
		})
	}
}

func TestDeleteFlashcard(t *testing.T) {
	repo := &fakeFlashcardRepository{}
	if httpErr := New(repo).DeleteFlashcard(context.Background(), "u1", 4); httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotUserID != "u1" || repo.gotID != 4 {
		t.Fatalf("bad repo args: %q %d", repo.gotUserID, repo.gotID)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "missing or not owned is 404", err: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "db error is 500", err: errDB, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := New(&fakeFlashcardRepository{err: tt.err}).DeleteFlashcard(context.Background(), "u1", 4)
			if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
				t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
			}
		})
	}
}
