package repository

import (
	"context"
	"time"

	"ego/services/notifications/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) GetNotifications(ctx context.Context, userID string, read *bool, limit, offset int) ([]*model.Notification, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	var unreadCount int64
	if err := base.Session(&gorm.Session{}).Where("read_at IS NULL").Count(&unreadCount).Error; err != nil {
		return nil, 0, 0, err
	}
	filtered := base.Session(&gorm.Session{})
	if read != nil {
		if *read {
			filtered = filtered.Where("read_at IS NOT NULL")
		} else {
			filtered = filtered.Where("read_at IS NULL")
		}
	}
	var total int64
	if err := filtered.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}
	var notifications []*model.Notification
	if err := filtered.Order("id DESC").Limit(limit).Offset(offset).Find(&notifications).Error; err != nil {
		return nil, 0, 0, err
	}
	return notifications, total, unreadCount, nil
}

func (r *Repository) UpsertDeviceToken(ctx context.Context, token *model.DeviceToken) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "token", "platform"}),
	}).Create(token).Error
}

func (r *Repository) DeleteDeviceToken(ctx context.Context, userID, token string) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND token = ?", userID, token).Delete(&model.DeviceToken{}).Error
}

func (r *Repository) DeleteDeviceTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("token IN ?", tokens).Delete(&model.DeviceToken{}).Error
}

func (r *Repository) GetDeviceTokens(ctx context.Context, userID string) ([]string, error) {
	var tokens []string
	err := r.db.WithContext(ctx).Model(&model.DeviceToken{}).Where("user_id = ?", userID).Pluck("token", &tokens).Error
	return tokens, err
}

func (r *Repository) MarkRead(ctx context.Context, userID string, ids []uint) error {
	query := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ? AND read_at IS NULL", userID)
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}
	return query.Update("read_at", time.Now().UTC()).Error
}

func (r *Repository) DeleteNotifications(ctx context.Context, userID string, ids []uint) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND id IN ?", userID, ids).Delete(&model.Notification{}).Error
}

func (r *Repository) CreateNotifications(ctx context.Context, notifications []*model.Notification) error {
	if len(notifications) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).Create(&notifications).Error
}
