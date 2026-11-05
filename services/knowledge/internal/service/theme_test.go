package service

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeThemeRepository struct {
	repository.Repository
	themes   []*model.Theme
	theme    *model.Theme
	words    []*model.Word
	total    int64
	themeErr error

	gotID     uint
	gotLimit  int32
	gotOffset int32
}

func (f *fakeThemeRepository) GetThemes(context.Context) ([]*model.Theme, error) {
	return f.themes, f.themeErr
}

func (f *fakeThemeRepository) GetThemeByID(_ context.Context, id uint, limit, offset int32) (*model.Theme, []*model.Word, int64, error) {
	f.gotID, f.gotLimit, f.gotOffset = id, limit, offset
	return f.theme, f.words, f.total, f.themeErr
}

func TestThemeModelsParse(t *testing.T) {
	cache := &sync.Map{}
	for _, m := range []any{&model.Theme{}, &model.ThemeWord{}} {
		parsed, err := schema.Parse(m, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
		if want := map[string]string{"Theme": "themes", "ThemeWord": "theme_words"}[parsed.Name]; parsed.Table != want {
			t.Fatalf("table of %s = %s, want %s", parsed.Name, parsed.Table, want)
		}
	}
}

func TestGetThemes(t *testing.T) {
	theme := &model.Theme{Category: "Daily", Name: "Travel", Icon: ptr("flight_takeoff"), WordCount: 3}
	theme.ID = 2
	got, httpErr := New(&fakeThemeRepository{themes: []*model.Theme{theme}}).GetThemes(context.Background())
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if len(got) != 1 || got[0].ID != 2 || *got[0].Icon != "flight_takeoff" || got[0].Description != nil || got[0].WordCount != 3 {
		t.Fatalf("bad mapping: %#v", got)
	}

	empty, _ := New(&fakeThemeRepository{}).GetThemes(context.Background())
	if empty == nil {
		t.Fatal("empty list must be a non-nil slice")
	}

	if _, httpErr := New(&fakeThemeRepository{themeErr: errDB}).GetThemes(context.Background()); httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", httpErr)
	}
}

func TestGetWordsByTheme(t *testing.T) {
	known := &model.Word{Word: "airport", Phonetic: ptr("/ˈeə.pɔːt/")}
	known.ID = 5
	unknown := &model.Word{Word: "jet lag"}

	repo := &fakeThemeRepository{theme: &model.Theme{Model: gorm.Model{ID: 2}, Name: "Travel"}, words: []*model.Word{known, unknown}, total: 12}
	got, httpErr := New(repo).GetWordsByTheme(context.Background(), 2, httpx.PaginationQuery{Page: 2, PageSize: 5})
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotID != 2 || repo.gotLimit != 5 || repo.gotOffset != 5 {
		t.Fatalf("bad repo args: %d %d %d", repo.gotID, repo.gotLimit, repo.gotOffset)
	}
	if got.Topic.ID != 2 || got.Page != 2 || got.PageCounts != 3 || got.Total != 12 {
		t.Fatalf("bad response: %#v", got)
	}
	if got.Data[0].ID != 5 || *got.Data[0].Phonetic != "/ˈeə.pɔːt/" || got.Data[1].ID != 0 || got.Data[1].Phonetic != nil {
		t.Fatalf("bad words: %#v %#v", got.Data[0], got.Data[1])
	}

	tests := []struct {
		name       string
		repo       *fakeThemeRepository
		wantStatus int
	}{
		{name: "missing theme is 404", repo: &fakeThemeRepository{themeErr: gorm.ErrRecordNotFound}, wantStatus: http.StatusNotFound},
		{name: "db error is 500", repo: &fakeThemeRepository{themeErr: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpErr := New(tt.repo).GetWordsByTheme(context.Background(), 2, httpx.PaginationQuery{Page: 1, PageSize: 10})
			if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
				t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
			}
		})
	}
}
