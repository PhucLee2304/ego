package service

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeGrammarRepository struct {
	repository.Repository
	units []*model.GrammarUnit
	unit  *model.GrammarUnit
	err   error

	gotUserID string
	gotID     uint
	upserted  *model.UserUnitProgress
}

func (f *fakeGrammarRepository) GetUnits(_ context.Context, userID string) ([]*model.GrammarUnit, error) {
	f.gotUserID = userID
	return f.units, f.err
}

func (f *fakeGrammarRepository) GetUnitByID(_ context.Context, userID string, id uint) (*model.GrammarUnit, error) {
	f.gotUserID, f.gotID = userID, id
	return f.unit, f.err
}

func (f *fakeGrammarRepository) UpsertProgress(_ context.Context, progress *model.UserUnitProgress) error {
	f.upserted = progress
	if f.err != nil {
		return f.err
	}
	progress.ID = 8
	return nil
}

func TestGrammarModelsParse(t *testing.T) {
	cache := &sync.Map{}
	for _, m := range []any{&model.GrammarUnit{}, &model.GrammarExample{}, &model.GrammarPractice{}, &model.UserUnitProgress{}} {
		if _, err := schema.Parse(m, cache, schema.NamingStrategy{}); err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
	}
}

func TestGetUnits(t *testing.T) {
	done := &model.GrammarUnit{Unit: 1, Title: "Pronouns", Progresses: []model.UserUnitProgress{{Completed: true}}}
	done.ID = 1
	todo := &model.GrammarUnit{Unit: 2, Title: "Articles"}
	todo.ID = 2

	repo := &fakeGrammarRepository{units: []*model.GrammarUnit{done, todo}}
	got, httpErr := New(repo).GetUnits(context.Background(), "u1")
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotUserID != "u1" || got.Total != 2 || len(got.Data) != 2 {
		t.Fatalf("bad response: %#v", got)
	}
	if !got.Data[0].Completed || got.Data[0].Unit != 1 || got.Data[0].Total != 1 || got.Data[1].Completed {
		t.Fatalf("bad summaries: %#v %#v", got.Data[0], got.Data[1])
	}

	empty, _ := New(&fakeGrammarRepository{}).GetUnits(context.Background(), "u1")
	if empty.Data == nil || empty.Total != 0 {
		t.Fatalf("empty unit list must have non-nil data: %#v", empty)
	}

	if _, httpErr := New(&fakeGrammarRepository{err: errDB}).GetUnits(context.Background(), "u1"); httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", httpErr)
	}
}

func TestGetUnitByID(t *testing.T) {
	unit := &model.GrammarUnit{
		Unit:      1,
		Title:     "Pronouns",
		Theory:    "I, you, he…",
		Examples:  []model.GrammarExample{{Content: "I am here."}},
		Practices: []model.GrammarPractice{{Question: "___ am", Options: []string{"I", "He"}, Correct: "I", Explanation: "first person", SortOrder: 1}},
	}
	unit.ID = 3

	repo := &fakeGrammarRepository{unit: unit}
	got, httpErr := New(repo).GetUnitByID(context.Background(), "u1", 3)
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotUserID != "u1" || repo.gotID != 3 {
		t.Fatalf("bad repo args: %q %d", repo.gotUserID, repo.gotID)
	}
	if got.ID != 3 || got.Theory != "I, you, he…" || got.Completed || len(got.Examples) != 1 || got.Examples[0] != "I am here." {
		t.Fatalf("bad unit: %#v", got)
	}
	practice := got.Practices[0]
	if practice.Question != "___ am" || len(practice.Options) != 2 || practice.Correct != "I" || practice.SortOrder != 1 {
		t.Fatalf("bad practice: %#v", practice)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "missing unit is 404", err: gorm.ErrRecordNotFound, wantStatus: http.StatusNotFound},
		{name: "db error is 500", err: errDB, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpErr := New(&fakeGrammarRepository{err: tt.err}).GetUnitByID(context.Background(), "u1", 3)
			if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
				t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
			}
		})
	}
}

func TestUpdateProgress(t *testing.T) {
	repo := &fakeGrammarRepository{}
	got, httpErr := New(repo).UpdateProgress(context.Background(), "u1", 3, dto.UpdateProgressBody{Completed: true})
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.upserted.UserID != "u1" || repo.upserted.UnitID != 3 || !repo.upserted.Completed {
		t.Fatalf("bad upserted progress: %#v", repo.upserted)
	}
	if got.ID != 8 || got.UserID != "u1" || got.UnitID != 3 || !got.Completed {
		t.Fatalf("bad response: %#v", got)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "missing unit is 404", err: gorm.ErrForeignKeyViolated, wantStatus: http.StatusNotFound},
		{name: "db error is 500", err: errDB, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpErr := New(&fakeGrammarRepository{err: tt.err}).UpdateProgress(context.Background(), "u1", 3, dto.UpdateProgressBody{})
			if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
				t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
			}
		})
	}
}
