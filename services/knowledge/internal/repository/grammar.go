package repository

import (
	"context"

	"ego/services/knowledge/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetUnits lấy mọi unit theo số thứ tự, kèm tiến độ của người dùng.
func (r *repository) GetUnits(ctx context.Context, userID string) ([]*model.GrammarUnit, error) {
	var units []*model.GrammarUnit
	if err := r.db.WithContext(ctx).
		Preload("Progresses", "user_id = ?", userID).
		Order("unit ASC").
		Find(&units).Error; err != nil {
		return nil, err
	}
	return units, nil
}

// GetUnitByID lấy unit kèm ví dụ, câu hỏi luyện tập và tiến độ của người dùng.
func (r *repository) GetUnitByID(ctx context.Context, userID string, id uint) (*model.GrammarUnit, error) {
	var unit model.GrammarUnit
	if err := r.db.WithContext(ctx).
		Preload("Examples", func(db *gorm.DB) *gorm.DB {
			return db.Order("id")
		}).
		Preload("Practices", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order").Order("id")
		}).
		Preload("Progresses", "user_id = ?", userID).
		First(&unit, id).Error; err != nil {
		return nil, err
	}
	return &unit, nil
}

// UpsertProgress tạo hoặc cập nhật tiến độ theo cặp (user_id, unit_id). Unit
// không tồn tại thì trả gorm.ErrForeignKeyViolated (DB bật TranslateError).
func (r *repository) UpsertProgress(ctx context.Context, progress *model.UserUnitProgress) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "unit_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"completed",
			"updated_at",
			"deleted_at",
		}),
	}).Omit("Unit").Create(progress).Error
}
