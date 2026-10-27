package repository

import (
	"context"
	"ego/services/dictionary/internal/model"
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("word not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// LookupWord returns a fully preloaded Word by exact (case-insensitive) match.
func (r *Repository) LookupWord(ctx context.Context, word string) (*model.Word, error) {
	var w model.Word
	err := r.db.WithContext(ctx).
		Where("LOWER(word) = LOWER(?)", word).
		Preload("Phonetics").
		Preload("Meanings").
		Preload("Meanings.Definitions").
		Preload("Meanings.Definitions.Synonyms").
		Preload("Meanings.Definitions.Antonyms").
		Preload("Meanings.Synonyms").
		Preload("Meanings.Antonyms").
		First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &w, nil
}

// SearchWords performs a prefix/ILIKE search with pagination.
func (r *Repository) SearchWords(ctx context.Context, query string, limit, offset int) ([]*model.Word, int64, error) {
	var words []*model.Word
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Word{})
	if query != "" {
		q = q.Where("LOWER(word) LIKE LOWER(?)", query+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := q.Order("word ASC").Limit(limit).Offset(offset).Find(&words).Error; err != nil {
		return nil, 0, err
	}

	return words, total, nil
}

// GetThesaurus aggregates all synonyms and antonyms for a word across all meanings.
func (r *Repository) GetThesaurus(ctx context.Context, wordID uint) (synonyms []string, antonyms []string, err error) {
	// meaning-level synonyms
	var mSyns []model.MeaningSynonym
	if err := r.db.WithContext(ctx).Where("meaning_id IN (SELECT id FROM meanings WHERE word_id = ? AND deleted_at IS NULL)", wordID).Find(&mSyns).Error; err != nil {
		return nil, nil, err
	}
	// definition-level synonyms
	var dSyns []model.DefinitionSynonym
	if err := r.db.WithContext(ctx).
		Where("definition_id IN (SELECT d.id FROM definitions d JOIN meanings m ON d.meaning_id = m.id WHERE m.word_id = ? AND d.deleted_at IS NULL AND m.deleted_at IS NULL)", wordID).
		Find(&dSyns).Error; err != nil {
		return nil, nil, err
	}
	// meaning-level antonyms
	var mAnts []model.MeaningAntonym
	if err := r.db.WithContext(ctx).Where("meaning_id IN (SELECT id FROM meanings WHERE word_id = ? AND deleted_at IS NULL)", wordID).Find(&mAnts).Error; err != nil {
		return nil, nil, err
	}
	// definition-level antonyms
	var dAnts []model.DefinitionAntonym
	if err := r.db.WithContext(ctx).
		Where("definition_id IN (SELECT d.id FROM definitions d JOIN meanings m ON d.meaning_id = m.id WHERE m.word_id = ? AND d.deleted_at IS NULL AND m.deleted_at IS NULL)", wordID).
		Find(&dAnts).Error; err != nil {
		return nil, nil, err
	}

	seen := make(map[string]struct{})
	for _, s := range mSyns {
		if _, ok := seen[s.Word]; !ok {
			synonyms = append(synonyms, s.Word)
			seen[s.Word] = struct{}{}
		}
	}
	for _, s := range dSyns {
		if _, ok := seen[s.Word]; !ok {
			synonyms = append(synonyms, s.Word)
			seen[s.Word] = struct{}{}
		}
	}

	seenA := make(map[string]struct{})
	for _, a := range mAnts {
		if _, ok := seenA[a.Word]; !ok {
			antonyms = append(antonyms, a.Word)
			seenA[a.Word] = struct{}{}
		}
	}
	for _, a := range dAnts {
		if _, ok := seenA[a.Word]; !ok {
			antonyms = append(antonyms, a.Word)
			seenA[a.Word] = struct{}{}
		}
	}

	if synonyms == nil {
		synonyms = []string{}
	}
	if antonyms == nil {
		antonyms = []string{}
	}
	return synonyms, antonyms, nil
}

// GetRandomWords returns n random words (used for flashcard quiz mode).
func (r *Repository) GetRandomWords(ctx context.Context, n int) ([]*model.Word, error) {
	var words []*model.Word
	if err := r.db.WithContext(ctx).Order("RANDOM()").Limit(n).Find(&words).Error; err != nil {
		return nil, err
	}
	return words, nil
}

// ---- Flashcard methods ----

func (r *Repository) GetFlashcards(ctx context.Context, userID string, limit, offset int) ([]*model.UserFlashcard, int64, error) {
	var cards []*model.UserFlashcard
	var total int64

	q := r.db.WithContext(ctx).Model(&model.UserFlashcard{}).Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := q.Preload("Word").Order("created_at DESC").Limit(limit).Offset(offset).Find(&cards).Error; err != nil {
		return nil, 0, err
	}
	return cards, total, nil
}

func (r *Repository) AddFlashcard(ctx context.Context, userID string, wordID uint, note string) (*model.UserFlashcard, error) {
	// check word exists
	var w model.Word
	if err := r.db.WithContext(ctx).First(&w, wordID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	card := &model.UserFlashcard{
		UserID: userID,
		WordID: wordID,
		Note:   note,
	}
	if err := r.db.WithContext(ctx).Create(card).Error; err != nil {
		return nil, err
	}
	card.Word = w
	return card, nil
}

func (r *Repository) DeleteFlashcard(ctx context.Context, userID string, flashcardID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", flashcardID, userID).
		Delete(&model.UserFlashcard{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) WordExists(ctx context.Context, word string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Word{}).Where("LOWER(word) = LOWER(?)", word).Count(&count).Error
	return count > 0, err
}
