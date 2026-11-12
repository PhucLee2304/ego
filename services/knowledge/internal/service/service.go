package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/repository"
)

// Service gom nghiệp vụ của mọi domain trong knowledge (topic, dictionary, grammar).
// Method của từng domain được nhóm bằng comment `// Domain` và cài đặt ở <domain>.go.
// Method phục vụ HTTP trả *httpx.HTTPError, method nội bộ trả error với sentinel.
type Service interface {
	// Topic (topic → section → lesson → transcript)
	GetTopics(ctx context.Context) (*dto.GetTopicsResponse, *httpx.HTTPError)
	GetSectionsByTopic(ctx context.Context, topicID uint) (*dto.GetSectionsResponse, *httpx.HTTPError)
	GetLessonsBySection(ctx context.Context, sectionID uint) (*dto.GetLessonsResponse, *httpx.HTTPError)
	GetLessonByID(ctx context.Context, id uint) (*dto.GetLessonByIDResponse, *httpx.HTTPError)

	// Word
	GetWords(ctx context.Context, query dto.GetWordsQuery) ([]*dto.Word, int64, *httpx.HTTPError)
	GetWordByID(ctx context.Context, id uint) (*dto.Word, *httpx.HTTPError)

	// Flashcard Deck
	CreateDeck(ctx context.Context, userID string, body dto.CreateDeckBody) (*dto.DeckResponse, *httpx.HTTPError)
	GetDecks(ctx context.Context, userID string) ([]*dto.DeckResponse, *httpx.HTTPError)
	GetDeckByID(ctx context.Context, userID string, id uint) (*dto.DeckResponse, *httpx.HTTPError)
	UpdateDeck(ctx context.Context, userID string, id uint, body dto.UpdateDeckBody) (*dto.DeckResponse, *httpx.HTTPError)
	DeleteDeck(ctx context.Context, userID string, id uint) *httpx.HTTPError

	// Flashcard Card & Study
	SaveWordToDecks(ctx context.Context, userID string, body dto.SaveWordToDecksBody) *httpx.HTTPError
	GetWordSaveStatus(ctx context.Context, userID string, wordID uint) (*dto.WordSaveStatusResponse, *httpx.HTTPError)
	GetCardsByDeck(ctx context.Context, userID string, deckID uint, query httpx.PaginationQuery) ([]*dto.Flashcard, int64, *httpx.HTTPError)
	UpdateFlashcardNote(ctx context.Context, userID string, id uint, note *string) (*dto.Flashcard, *httpx.HTTPError)
	DeleteFlashcard(ctx context.Context, userID string, id uint) *httpx.HTTPError
	GetStudyCards(ctx context.Context, userID string, deckID uint) ([]*dto.Flashcard, *httpx.HTTPError)
	GetAllDueCards(ctx context.Context, userID string) ([]*dto.Flashcard, *httpx.HTTPError)
	ReviewFlashcard(ctx context.Context, userID string, id uint, body dto.ReviewCardBody) (*dto.ReviewCardResponse, *httpx.HTTPError)
	GetFlashcardUserStats(ctx context.Context, userID string) (*dto.FlashcardUserStats, *httpx.HTTPError)

	// Theme
	GetThemes(ctx context.Context) ([]*dto.Theme, *httpx.HTTPError)
	GetWordsByTheme(ctx context.Context, id uint, query httpx.PaginationQuery) (*dto.GetThemeWordsResponse, *httpx.HTTPError)

	// Grammar
	GetUnits(ctx context.Context, userID string) (*dto.GetUnitsResponse, *httpx.HTTPError)
	GetUnitByID(ctx context.Context, userID string, id uint) (*dto.Unit, *httpx.HTTPError)
	UpdateProgress(ctx context.Context, userID string, id uint, body dto.UpdateProgressBody) (*dto.Progress, *httpx.HTTPError)

	// Stats (admin)
	GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError)
}

type service struct {
	repo repository.Repository
}

func New(repo repository.Repository) Service {
	return &service{repo: repo}
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}
