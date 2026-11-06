package repository

import (
	"context"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

// Repository gom truy vấn của mọi domain trong knowledge (topic, dictionary, grammar).
// Method của từng domain được nhóm bằng comment `// Domain` và cài đặt ở <domain>.go.
type Repository interface {
	// Topic (topic → section → lesson → transcript)
	GetTopics(ctx context.Context) ([]*model.Topic, error)
	GetTopicByID(ctx context.Context, id uint) (*model.Topic, error)
	GetSectionsByTopic(ctx context.Context, topicID uint) ([]*model.Section, error)
	GetLessonsBySection(ctx context.Context, sectionID uint) ([]*model.Lesson, error)
	GetLessonByID(ctx context.Context, id uint) (*model.Lesson, error)

	// Word
	GetWords(ctx context.Context, query string, limit, offset int32) ([]*model.Word, int64, error)
	GetWordByID(ctx context.Context, id uint) (*model.Word, error)

	// Flashcard
	GetFlashcards(ctx context.Context, userID string, limit, offset int32) ([]*model.Flashcard, int64, error)
	CreateFlashcard(ctx context.Context, flashcard *model.Flashcard) error
	DeleteFlashcard(ctx context.Context, userID string, id uint) error

	// Theme
	GetThemes(ctx context.Context) ([]*model.Theme, error)
	GetThemeByID(ctx context.Context, id uint, limit, offset int32) (*model.Theme, []*model.Word, int64, error)

	// Grammar
	GetUnits(ctx context.Context, userID string) ([]*model.GrammarUnit, error)
	GetUnitByID(ctx context.Context, userID string, id uint) (*model.GrammarUnit, error)
	UpsertProgress(ctx context.Context, progress *model.UserUnitProgress) error

	// Stats (admin)
	GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error)

	Transaction(ctx context.Context, fn func(Repository) error) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}
