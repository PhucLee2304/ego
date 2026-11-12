package repository

import (
	"context"

	"ego/services/notifications/internal/model"

	"gorm.io/gorm/clause"
)

// UpsertDeviceToken lưu token push của một thiết bị. Một token FCM chỉ thuộc một bản cài đặt trình
// duyệt/app, nên bản ghi khác đang giữ token này (deviceId cũ bị mất, hoặc user trước chưa xoá được
// khi đăng xuất) bị xoá trước; sau đó upsert theo device_id để thiết bị chuyển sang user hiện tại.
// Nên gọi trong Transaction để hai bước không bị request khác chen giữa.
// **Reviewed**
func (r *repository) UpsertDeviceToken(ctx context.Context, token *model.DeviceToken) error {
	db := r.db.WithContext(ctx)
	if err := db.Where("token = ? AND device_id <> ?", token.Token, token.DeviceID).Delete(&model.DeviceToken{}).Error; err != nil {
		return err
	}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "token", "platform"}),
	}).Create(token).Error
}

// **Reviewed**
func (r *repository) DeleteDeviceToken(ctx context.Context, userID, token string) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND token = ?", userID, token).Delete(&model.DeviceToken{}).Error
}

// **Reviewed**
func (r *repository) DeleteDeviceTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("token IN ?", tokens).Delete(&model.DeviceToken{}).Error
}

// **Reviewed**
func (r *repository) GetDeviceTokens(ctx context.Context, userIDs []string) ([]string, error) {
	var tokens []string
	err := r.db.WithContext(ctx).Model(&model.DeviceToken{}).Where("user_id IN ?", userIDs).Pluck("token", &tokens).Error
	return tokens, err
}

// **Reviewed**
func (r *repository) DeleteDeviceTokensByUserID(ctx context.Context, userID string) (int64, error) {
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&model.DeviceToken{})
	return result.RowsAffected, result.Error
}
