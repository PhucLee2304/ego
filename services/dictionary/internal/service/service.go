package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"ego/services/dictionary/internal/dto"
	"ego/services/dictionary/internal/model"
	"ego/services/dictionary/internal/repository"
	"strings"
)

var ErrNotFound = errors.New("word not found")

type Service interface {
	LookupWord(ctx context.Context, word string) (*dto.WordResponse, error)
	SearchWords(ctx context.Context, q string, page, pageSize int) (*dto.SearchResponse, error)
	GetThesaurus(ctx context.Context, word string) (*dto.ThesaurusResponse, error)
	GetFlashcards(ctx context.Context, userID string, page, pageSize int) (*dto.GetFlashcardsResponse, error)
	AddFlashcard(ctx context.Context, userID string, wordID uint, note string) (*dto.FlashcardResponse, error)
	DeleteFlashcard(ctx context.Context, userID string, flashcardID uint) error
}

type service struct {
	repo         *repository.Repository
	audioBaseURL string // e.g. "/dictionary/assets"
}

func New(repo *repository.Repository, audioBaseURL string) Service {
	return &service{repo: repo, audioBaseURL: audioBaseURL}
}

// ---- Word Lookup ----

func (s *service) LookupWord(ctx context.Context, word string) (*dto.WordResponse, error) {
	w, err := s.repo.LookupWord(ctx, word)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.mapWordToResponse(w), nil
}

// ---- Search ----

func (s *service) SearchWords(ctx context.Context, q string, page, pageSize int) (*dto.SearchResponse, error) {
	limit := pageSize
	offset := (page - 1) * pageSize

	words, total, err := s.repo.SearchWords(ctx, q, limit, offset)
	if err != nil {
		return nil, err
	}

	summaries := make([]dto.WordSummaryResponse, 0, len(words))
	for _, w := range words {
		summaries = append(summaries, dto.WordSummaryResponse{
			ID:       w.ID,
			Word:     w.Word,
			Phonetic: w.Phonetic,
		})
	}

	pageCounts := int(math.Ceil(float64(total) / float64(pageSize)))

	return &dto.SearchResponse{
		Data:       summaries,
		Page:       page,
		PageCounts: pageCounts,
		Total:      total,
	}, nil
}

// ---- Thesaurus ----

func (s *service) GetThesaurus(ctx context.Context, word string) (*dto.ThesaurusResponse, error) {
	w, err := s.repo.LookupWord(ctx, word)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	synonyms, antonyms, err := s.repo.GetThesaurus(ctx, w.ID)
	if err != nil {
		return nil, err
	}

	return &dto.ThesaurusResponse{
		Word:     w.Word,
		Synonyms: synonyms,
		Antonyms: antonyms,
	}, nil
}

// ---- Flashcards ----

func (s *service) GetFlashcards(ctx context.Context, userID string, page, pageSize int) (*dto.GetFlashcardsResponse, error) {
	limit := pageSize
	offset := (page - 1) * pageSize

	cards, total, err := s.repo.GetFlashcards(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]dto.FlashcardResponse, 0, len(cards))
	for _, c := range cards {
		items = append(items, s.mapFlashcardToResponse(c))
	}

	pageCounts := int(math.Ceil(float64(total) / float64(pageSize)))

	return &dto.GetFlashcardsResponse{
		Data:       items,
		Page:       page,
		PageCounts: pageCounts,
		Total:      total,
	}, nil
}

func (s *service) AddFlashcard(ctx context.Context, userID string, wordID uint, note string) (*dto.FlashcardResponse, error) {
	card, err := s.repo.AddFlashcard(ctx, userID, wordID, note)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	resp := s.mapFlashcardToResponse(card)
	return &resp, nil
}

func (s *service) DeleteFlashcard(ctx context.Context, userID string, flashcardID uint) error {
	err := s.repo.DeleteFlashcard(ctx, userID, flashcardID)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// ---- Mappers ----

func normalizeAudioPath(audioPath string) string {
	p := strings.TrimSpace(audioPath)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "assets")
	return strings.TrimPrefix(p, "/")
}

func (s *service) mapWordToResponse(w *model.Word) *dto.WordResponse {
	phonetics := make([]dto.PhoneticResponse, 0, len(w.Phonetics))
	for _, p := range w.Phonetics {
		pr := dto.PhoneticResponse{
			Text:      p.Text,
			SourceURL: p.SourceURL,
		}
		if p.AudioPath != "" {
			pr.AudioURL = fmt.Sprintf("%s/%s",
				strings.TrimRight(s.audioBaseURL, "/"),
				normalizeAudioPath(p.AudioPath),
			)
		}
		phonetics = append(phonetics, pr)
	}

	meanings := make([]dto.MeaningResponse, 0, len(w.Meanings))
	for _, m := range w.Meanings {
		defs := make([]dto.DefinitionResponse, 0, len(m.Definitions))
		for _, d := range m.Definitions {
			defSyns := make([]string, 0, len(d.Synonyms))
			for _, ds := range d.Synonyms {
				defSyns = append(defSyns, ds.Word)
			}
			defAnts := make([]string, 0, len(d.Antonyms))
			for _, da := range d.Antonyms {
				defAnts = append(defAnts, da.Word)
			}
			defs = append(defs, dto.DefinitionResponse{
				Definition: d.Definition,
				Example:    d.Example,
				Synonyms:   defSyns,
				Antonyms:   defAnts,
			})
		}

		mSyns := make([]string, 0, len(m.Synonyms))
		for _, ms := range m.Synonyms {
			mSyns = append(mSyns, ms.Word)
		}
		mAnts := make([]string, 0, len(m.Antonyms))
		for _, ma := range m.Antonyms {
			mAnts = append(mAnts, ma.Word)
		}

		meanings = append(meanings, dto.MeaningResponse{
			PartOfSpeech: m.PartOfSpeech,
			Definitions:  defs,
			Synonyms:     mSyns,
			Antonyms:     mAnts,
		})
	}

	// Decode source URLs
	var sourceURLs []string
	if w.SourceURLs != "" {
		_ = json.Unmarshal([]byte(w.SourceURLs), &sourceURLs)
	}
	if sourceURLs == nil {
		sourceURLs = []string{}
	}

	return &dto.WordResponse{
		ID:          w.ID,
		Word:        w.Word,
		Phonetic:    w.Phonetic,
		Phonetics:   phonetics,
		Meanings:    meanings,
		LicenseName: w.LicenseName,
		LicenseURL:  w.LicenseURL,
		SourceURLs:  sourceURLs,
	}
}

func (s *service) mapFlashcardToResponse(c *model.UserFlashcard) dto.FlashcardResponse {
	return dto.FlashcardResponse{
		ID:        c.ID,
		Note:      c.Note,
		CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Word: dto.WordSummaryResponse{
			ID:       c.Word.ID,
			Word:     c.Word.Word,
			Phonetic: c.Word.Phonetic,
		},
	}
}
