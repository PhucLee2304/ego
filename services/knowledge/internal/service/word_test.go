package service

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"
	"ego/services/knowledge/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type fakeWordRepository struct {
	repository.Repository
	word      *model.Word
	wordErr   error
	words     []*model.Word
	total     int64
	wordsErr  error
	gotQuery  string
	gotLimit  int32
	gotOffset int32
}

func (f *fakeWordRepository) GetWords(_ context.Context, query string, limit, offset int32) ([]*model.Word, int64, error) {
	f.gotQuery, f.gotLimit, f.gotOffset = query, limit, offset
	return f.words, f.total, f.wordsErr
}

func (f *fakeWordRepository) GetWordByID(context.Context, uint) (*model.Word, error) {
	return f.word, f.wordErr
}

func ptr(value string) *string { return &value }

func TestWordModelsParse(t *testing.T) {
	models := []any{&model.Word{}, &model.Phonetic{}, &model.Meaning{}, &model.Definition{}}
	for _, m := range models {
		if _, err := schema.Parse(m, &sync.Map{}, schema.NamingStrategy{}); err != nil {
			t.Fatalf("parse %T: %v", m, err)
		}
	}
}

func TestGetWords(t *testing.T) {
	word := &model.Word{Word: "hello", Phonetic: ptr("/h/")}
	word.ID = 1
	query := dto.GetWordsQuery{Query: "He", PaginationQuery: httpx.PaginationQuery{Page: 3, PageSize: 10}}

	repo := &fakeWordRepository{words: []*model.Word{word}, total: 21}
	words, total, httpErr := New(repo).GetWords(context.Background(), query)
	if httpErr != nil {
		t.Fatalf("unexpected error: %v", httpErr)
	}
	if repo.gotQuery != "He" || repo.gotLimit != 10 || repo.gotOffset != 20 {
		t.Fatalf("bad repo args: %q %d %d", repo.gotQuery, repo.gotLimit, repo.gotOffset)
	}
	if total != 21 || len(words) != 1 || words[0].ID != 1 || words[0].Word != "hello" || *words[0].Phonetic != "/h/" {
		t.Fatalf("bad result: %#v total=%d", words, total)
	}
	if words[0].Phonetics == nil || len(words[0].Phonetics) != 0 || words[0].Meanings == nil || len(words[0].Meanings) != 0 {
		t.Fatalf("list items must have empty (non-nil) phonetics and meanings: %#v", words[0])
	}

	empty, _, _ := New(&fakeWordRepository{}).GetWords(context.Background(), query)
	if empty == nil {
		t.Fatal("empty list must be a non-nil slice")
	}

	if _, _, httpErr := New(&fakeWordRepository{wordsErr: errDB}).GetWords(context.Background(), query); httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", httpErr)
	}
}

func TestGetWordByID(t *testing.T) {
	word := &model.Word{
		Word:       "hello",
		Phonetic:   ptr("/həˈləʊ/"),
		SourceUrls: datatypes.JSONSlice[string]{"https://src"},
		Phonetics:  []model.Phonetic{{Text: ptr("/h/"), AudioUrl: ptr("/ego/dictionary/hello.mp3")}, {}},
		// Mảng rỗng tường minh như dữ liệu thật trong DB (CHECK jsonb_typeof = 'array').
		Meanings: []model.Meaning{{
			PartOfSpeech: model.PartOfSpeechNoun,
			Synonyms:     datatypes.JSONSlice[string]{"hi"},
			Antonyms:     datatypes.JSONSlice[string]{},
			Definitions: []model.Definition{
				{DefinitionText: "a greeting", Example: ptr("hello there"), Synonyms: datatypes.JSONSlice[string]{}, Antonyms: datatypes.JSONSlice[string]{"bye"}},
				{DefinitionText: "no example", Synonyms: datatypes.JSONSlice[string]{}, Antonyms: datatypes.JSONSlice[string]{}},
			},
		}},
	}
	word.ID = 5

	tests := []struct {
		name       string
		repo       *fakeWordRepository
		wantStatus int
	}{
		{name: "maps full word", repo: &fakeWordRepository{word: word}},
		{name: "missing word is 404", repo: &fakeWordRepository{wordErr: gorm.ErrRecordNotFound}, wantStatus: http.StatusNotFound},
		{name: "db error is 500", repo: &fakeWordRepository{wordErr: errDB}, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, httpErr := New(tt.repo).GetWordByID(context.Background(), 5)
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("want status %d, got %v", tt.wantStatus, httpErr)
				}
				return
			}
			if httpErr != nil {
				t.Fatalf("unexpected error: %v", httpErr)
			}
			if got.ID != 5 || *got.Phonetic != "/həˈləʊ/" || !reflect.DeepEqual(got.SourceURLs, []string{"https://src"}) {
				t.Fatalf("bad word mapping: %#v", got)
			}
			if len(got.Phonetics) != 2 || *got.Phonetics[0].AudioURL != "/ego/dictionary/hello.mp3" || got.Phonetics[1].Text != nil {
				t.Fatalf("bad phonetics: %#v", got.Phonetics)
			}
			meaning := got.Meanings[0]
			if meaning.PartOfSpeech != "noun" || !reflect.DeepEqual(meaning.Synonyms, []string{"hi"}) || meaning.Antonyms == nil {
				t.Fatalf("bad meaning: %#v", meaning)
			}
			first, second := meaning.Definitions[0], meaning.Definitions[1]
			if first.Definition != "a greeting" || *first.Example != "hello there" || !reflect.DeepEqual(first.Antonyms, []string{"bye"}) || first.Synonyms == nil {
				t.Fatalf("bad definition: %#v", first)
			}
			if second.Example != nil || second.Synonyms == nil || second.Antonyms == nil {
				t.Fatalf("definition without example must have nil example and empty lists: %#v", second)
			}
		})
	}
}

func TestWordNullableJSON(t *testing.T) {
	raw, _ := json.Marshal(dto.ToWord(&model.Word{Word: "x", SourceUrls: datatypes.JSONSlice[string]{}}))
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	if value, ok := body["phonetic"]; !ok || value != nil {
		t.Fatalf("phonetic must be present and null, got %v (present=%v)", value, ok)
	}
	if urls, ok := body["sourceUrls"].([]any); !ok || len(urls) != 0 {
		t.Fatalf("sourceUrls must be an empty array, got %#v", body["sourceUrls"])
	}
}
