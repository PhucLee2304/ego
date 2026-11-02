package repository

import (
	"context"
	"errors"

	platformdb "ego/platform/db"
	"ego/services/vocabgram/internal/dto"
	"ego/services/vocabgram/internal/model"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("record not found")

type Repository interface {
	Transaction(ctx context.Context, fn func(Repository) error) error

	// Dictionary
	LookupWord(ctx context.Context, word string) (*model.Word, error)
	SearchWords(ctx context.Context, query string, limit, offset int) ([]*model.Word, int64, error)
	GetThesaurus(ctx context.Context, wordID uint) (synonyms []string, antonyms []string, err error)
	GetRandomWords(ctx context.Context, n int) ([]*model.Word, error)
	WordExists(ctx context.Context, word string) (bool, error)

	// Flashcards
	GetFlashcards(ctx context.Context, userID string, limit, offset int) ([]*model.UserFlashcard, int64, error)
	AddFlashcard(ctx context.Context, userID string, wordID uint, note string) (*model.UserFlashcard, error)
	DeleteFlashcard(ctx context.Context, userID string, flashcardID uint) error

	// Topics
	ListTopics(ctx context.Context) ([]model.Topic, error)
	GetTopicByID(ctx context.Context, id string) (*model.Topic, error)
	GetTopicWords(ctx context.Context, topicID string, limit, offset int) ([]dto.TopicWordItemResponse, int64, error)

	// Grammar
	ListUnits(ctx context.Context) ([]*model.GrammarUnit, error)
	GetUnitByID(ctx context.Context, id uint) (*model.GrammarUnit, error)
	UnitExists(ctx context.Context, id uint) (bool, error)
	GetProgress(ctx context.Context, userID string, unitID uint) (*model.UserUnitProgress, error)
	ListProgress(ctx context.Context, userID string) ([]*model.UserUnitProgress, error)
	UpsertProgress(ctx context.Context, userID string, unitID uint, completed bool) (*model.UserUnitProgress, error)
	CountUnits(ctx context.Context) (int64, error)
	BulkCreateUnits(ctx context.Context, units []*model.GrammarUnit) error
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

