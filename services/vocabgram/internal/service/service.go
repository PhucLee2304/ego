package service

import (
	"context"
	"errors"

	"ego/services/vocabgram/internal/dto"
	"ego/services/vocabgram/internal/repository"
)

var (
	ErrNotFound        = errors.New("record not found")
	ErrWordNotFound    = errors.New("word not found")
	ErrGrammarNotFound = errors.New("grammar unit not found")
)

type Service interface {
	// Dictionary
	LookupWord(ctx context.Context, word string) (*dto.WordResponse, error)
	SearchWords(ctx context.Context, q string, page, pageSize int) (*dto.SearchResponse, error)
	GetThesaurus(ctx context.Context, word string) (*dto.ThesaurusResponse, error)
	GetFlashcards(ctx context.Context, userID string, page, pageSize int) (*dto.GetFlashcardsResponse, error)
	AddFlashcard(ctx context.Context, userID string, wordID uint, note string) (*dto.FlashcardResponse, error)
	DeleteFlashcard(ctx context.Context, userID string, flashcardID uint) error
	ListTopics(ctx context.Context) ([]dto.TopicResponse, error)
	GetTopicWords(ctx context.Context, topicID string, page, pageSize int) (*dto.GetTopicWordsResponse, error)

	// Grammar
	GetCurriculum(ctx context.Context, userID string) (*dto.CurriculumResponse, error)
	GetUnit(ctx context.Context, userID string, unitID uint) (*dto.UnitDetailResponse, error)
	SubmitProgress(ctx context.Context, userID string, unitID uint, completed bool) (*dto.ProgressResponse, error)
}

type service struct {
	repo         repository.Repository
	audioBaseURL string // e.g. "/dictionary/assets"
}

func New(repo repository.Repository, audioBaseURL string) Service {
	return &service{
		repo:         repo,
		audioBaseURL: audioBaseURL,
	}
}

func (s *service) withRepo(repo repository.Repository) *service {
	return &service{
		repo:         repo,
		audioBaseURL: s.audioBaseURL,
	}
}

