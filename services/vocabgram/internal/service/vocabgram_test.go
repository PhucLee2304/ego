package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"ego/services/vocabgram/internal/model"
	"ego/services/vocabgram/internal/repository"

	"gorm.io/gorm"
)

type fakeRepo struct {
	repository.Repository
	word        *model.Word
	wordErr     error
	words       []*model.Word
	wordsTotal  int64
	synonyms    []string
	antonyms    []string
	units       []*model.GrammarUnit
	unit        *model.GrammarUnit
	unitErr     error
	unitExists  bool
	progress    []*model.UserUnitProgress
	userProg    *model.UserUnitProgress
	progressErr error
}

func (f *fakeRepo) LookupWord(ctx context.Context, word string) (*model.Word, error) {
	return f.word, f.wordErr
}

func (f *fakeRepo) SearchWords(ctx context.Context, query string, limit, offset int) ([]*model.Word, int64, error) {
	return f.words, f.wordsTotal, nil
}

func (f *fakeRepo) GetThesaurus(ctx context.Context, wordID uint) ([]string, []string, error) {
	return f.synonyms, f.antonyms, nil
}

func (f *fakeRepo) ListUnits(ctx context.Context) ([]*model.GrammarUnit, error) {
	return f.units, nil
}

func (f *fakeRepo) GetUnitByID(ctx context.Context, id uint) (*model.GrammarUnit, error) {
	return f.unit, f.unitErr
}

func (f *fakeRepo) UnitExists(ctx context.Context, id uint) (bool, error) {
	return f.unitExists, nil
}

func (f *fakeRepo) ListProgress(ctx context.Context, userID string) ([]*model.UserUnitProgress, error) {
	return f.progress, nil
}

func (f *fakeRepo) GetProgress(ctx context.Context, userID string, unitID uint) (*model.UserUnitProgress, error) {
	return f.userProg, nil
}

func (f *fakeRepo) UpsertProgress(ctx context.Context, userID string, unitID uint, completed bool) (*model.UserUnitProgress, error) {
	return &model.UserUnitProgress{
		Model:     gorm.Model{ID: 1, UpdatedAt: time.Now()},
		UserID:    userID,
		UnitID:    unitID,
		Completed: completed,
	}, f.progressErr
}

func TestLookupWord(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		repo := &fakeRepo{
			word: &model.Word{
				Model:    gorm.Model{ID: 1},
				Word:     "hello",
				Phonetic: "/həˈləʊ/",
				Phonetics: []model.Phonetic{
					{Text: "/həˈləʊ/", AudioPath: "assets/audios/hello.mp3"},
				},
				Meanings: []model.Meaning{
					{
						PartOfSpeech: "interjection",
						Definitions: []model.Definition{
							{Definition: "used as a greeting"},
						},
					},
				},
			},
		}
		svc := New(repo, "/dictionary/assets")
		resp, err := svc.LookupWord(context.Background(), "hello")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Word != "hello" {
			t.Errorf("expected word hello, got %s", resp.Word)
		}
		if len(resp.Phonetics) != 1 || resp.Phonetics[0].AudioURL != "/dictionary/assets/audios/hello.mp3" {
			t.Errorf("unexpected audio url: %v", resp.Phonetics)
		}
	})

	t.Run("not found", func(t *testing.T) {
		repo := &fakeRepo{wordErr: repository.ErrNotFound}
		svc := New(repo, "/dictionary/assets")
		_, err := svc.LookupWord(context.Background(), "missing")
		if !errors.Is(err, ErrWordNotFound) {
			t.Fatalf("expected ErrWordNotFound, got %v", err)
		}
	})
}

func TestGrammarCurriculum(t *testing.T) {
	repo := &fakeRepo{
		units: []*model.GrammarUnit{
			{Model: gorm.Model{ID: 1}, Unit: 1, Title: "Nouns"},
			{Model: gorm.Model{ID: 2}, Unit: 2, Title: "Verbs"},
		},
		progress: []*model.UserUnitProgress{
			{UnitID: 1, Completed: true},
		},
	}
	svc := New(repo, "/dictionary/assets")
	resp, err := svc.GetCurriculum(context.Background(), "user123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Total != 2 {
		t.Fatalf("expected total 2, got %d", resp.Total)
	}
	if !resp.Data[0].Completed {
		t.Errorf("expected unit 1 to be completed")
	}
	if resp.Data[1].Completed {
		t.Errorf("expected unit 2 to not be completed")
	}
}

func TestGrammarLessonAndProgress(t *testing.T) {
	repo := &fakeRepo{
		unit: &model.GrammarUnit{
			Model:  gorm.Model{ID: 1},
			Unit:   1,
			Title:  "Pronouns",
			Theory: "Pronouns replace nouns",
			Examples: []model.GrammarExample{
				{Content: "He is happy."},
			},
			Practices: []model.GrammarPractice{
				{
					Question:    "Choose pronoun",
					Options:     `["He","Run"]`,
					Correct:     "He",
					Explanation: "He is a pronoun",
				},
			},
		},
		unitExists: true,
	}
	svc := New(repo, "/dictionary/assets")

	unitResp, err := svc.GetUnit(context.Background(), "user123", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unitResp.Title != "Pronouns" {
		t.Errorf("expected title Pronouns, got %s", unitResp.Title)
	}
	if len(unitResp.Practices) != 1 || len(unitResp.Practices[0].Options) != 2 {
		t.Errorf("unexpected practices: %v", unitResp.Practices)
	}

	progResp, err := svc.SubmitProgress(context.Background(), "user123", 1, true)
	if err != nil {
		t.Fatalf("unexpected error submitting progress: %v", err)
	}
	if !progResp.Completed {
		t.Errorf("expected completed true")
	}
}
