package repository

import (
	"context"

	platformdb "ego/platform/db"
	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
)

// GetFlashcards lấy flashcard của người dùng kèm từ, mới nhất trước.
func (r *repository) GetFlashcards(ctx context.Context, userID string, limit, offset int32) ([]*model.Flashcard, int64, error) {
	var flashcards []*model.Flashcard
	var total int64

	db := r.db.WithContext(ctx).Model(&model.Flashcard{}).Where("user_id = ?", userID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Word").
		Order("created_at DESC").
		Order("id DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&flashcards).Error; err != nil {
		return nil, 0, err
	}
	return flashcards, total, nil
}

// CreateFlashcard tạo flashcard rồi nạp lại kèm từ. Từ không tồn tại thì trả
// gorm.ErrForeignKeyViolated (DB bật TranslateError).
func (r *repository) CreateFlashcard(ctx context.Context, flashcard *model.Flashcard) error {
	if err := r.db.WithContext(ctx).Omit("Word").Create(flashcard).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Preload("Word").First(flashcard, flashcard.ID).Error
}

// DeleteFlashcard xóa mềm flashcard của đúng người dùng trong một transaction.
// Không xóa được dòng nào (không có hoặc không phải của mình) thì rollback và
// trả gorm.ErrRecordNotFound.
func (r *repository) DeleteFlashcard(ctx context.Context, userID string, id uint) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Flashcard{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
