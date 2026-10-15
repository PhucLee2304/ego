package repository

import (
	"context"
	"errors"

	"ego/services/grammar/internal/model"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("record not found")

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ---- GrammarUnit ----

// ListUnits returns all grammar units ordered by unit number.
func (r *Repository) ListUnits(ctx context.Context) ([]*model.GrammarUnit, error) {
	var units []*model.GrammarUnit
	if err := r.db.WithContext(ctx).Order("unit ASC").Find(&units).Error; err != nil {
		return nil, err
	}
	return units, nil
}

// GetUnitByID returns a fully preloaded unit (examples + practices).
func (r *Repository) GetUnitByID(ctx context.Context, id uint) (*model.GrammarUnit, error) {
	var unit model.GrammarUnit
	err := r.db.WithContext(ctx).
		Preload("Examples").
		Preload("Practices", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		First(&unit, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &unit, nil
}

// UnitExists checks whether a unit with the given ID exists.
func (r *Repository) UnitExists(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GrammarUnit{}).Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

// ---- UserUnitProgress ----

// GetProgress returns the progress record for a user + unit pair, or nil if not started.
func (r *Repository) GetProgress(ctx context.Context, userID string, unitID uint) (*model.UserUnitProgress, error) {
	var p model.UserUnitProgress
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND unit_id = ?", userID, unitID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not started yet
		}
		return nil, err
	}
	return &p, nil
}

// ListProgress returns all progress records for a user (keyed by unitID).
func (r *Repository) ListProgress(ctx context.Context, userID string) ([]*model.UserUnitProgress, error) {
	var progress []*model.UserUnitProgress
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&progress).Error; err != nil {
		return nil, err
	}
	return progress, nil
}

// UpsertProgress creates or updates a user's progress for a unit.
func (r *Repository) UpsertProgress(ctx context.Context, userID string, unitID uint, completed bool) (*model.UserUnitProgress, error) {
	var p model.UserUnitProgress
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND unit_id = ?", userID, unitID).
		First(&p)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			// Create new record
			p = model.UserUnitProgress{
				UserID:    userID,
				UnitID:    unitID,
				Completed: completed,
			}
			if err := r.db.WithContext(ctx).Create(&p).Error; err != nil {
				return nil, err
			}
			return &p, nil
		}
		return nil, result.Error
	}

	// Update existing
	if err := r.db.WithContext(ctx).Model(&p).Update("completed", completed).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ---- Seeding helpers ----

// CountUnits returns the number of grammar units in the database.
func (r *Repository) CountUnits(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.GrammarUnit{}).Count(&count).Error
	return count, err
}

// BulkCreateUnits inserts units with their nested examples and practices.
func (r *Repository) BulkCreateUnits(ctx context.Context, units []*model.GrammarUnit) error {
	return r.db.WithContext(ctx).
		Session(&gorm.Session{FullSaveAssociations: true}).
		Create(&units).Error
}
