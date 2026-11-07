package repository

import (
	"context"
	"time"

	"ego/services/notifications/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GetNotifications trả một trang thông báo của user (mới nhất trước), kèm tổng số bản ghi khớp bộ lọc
// và tổng số chưa đọc (không tính bộ lọc). types rỗng (nil) là không lọc theo data.type.
// **Reviewed**
func (r *repository) GetNotifications(ctx context.Context, userID string, read *bool, types []string, limit, offset int) ([]*model.Notification, int64, int64, error) {
	base := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ?", userID)
	var unreadCount int64
	if err := base.Session(&gorm.Session{}).Where("read_at IS NULL").Count(&unreadCount).Error; err != nil {
		return nil, 0, 0, err
	}
	filtered := base.Session(&gorm.Session{})
	if types != nil {
		filtered = filtered.Where("data->>'type' IN ?", types)
	}
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

// CountUnreadByType đếm thông báo chưa đọc của user theo data.type.
// **Reviewed**
func (r *repository) CountUnreadByType(ctx context.Context, userID string) (map[string]int64, error) {
	var rows []struct {
		Type  string
		Count int64
	}
	err := r.db.WithContext(ctx).Model(&model.Notification{}).
		Select("data->>'type' AS type, COUNT(*) AS count").
		Where("user_id = ? AND read_at IS NULL", userID).
		Group("data->>'type'").Scan(&rows).Error
	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Type] = row.Count
	}
	return out, err
}

// MarkRead đánh dấu đã đọc các thông báo chưa đọc của user; ids rỗng là đánh dấu tất cả.
// **Reviewed**
func (r *repository) MarkRead(ctx context.Context, userID string, ids []uint) error {
	query := r.db.WithContext(ctx).Model(&model.Notification{}).Where("user_id = ? AND read_at IS NULL", userID)
	if len(ids) > 0 {
		query = query.Where("id IN ?", ids)
	}
	return query.Update("read_at", time.Now().UTC()).Error
}

// **Reviewed**
func (r *repository) DeleteNotifications(ctx context.Context, userID string, ids []uint) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND id IN ?", userID, ids).Delete(&model.Notification{}).Error
}

// **Reviewed**
func (r *repository) CreateNotifications(ctx context.Context, notifications []*model.Notification) ([]*model.Notification, error) {
	// Lô rỗng: không có gì để thêm, không chạy câu SQL nào.
	if len(notifications) == 0 {
		return nil, nil
	}

	// Bước 1: gom idempotency_key của cả lô để kiểm tra trùng bằng một truy vấn.
	// Lọc trước thay vì dựa vào ON CONFLICT DO NOTHING: GORM gán ID theo thứ tự dòng RETURNING,
	// bỏ qua dòng giữa chừng thì ID sẽ bị gán lệch sang bản khác.
	keys := make([]string, len(notifications))
	for i, notification := range notifications {
		keys[i] = notification.IdempotencyKey
	}

	// Bước 2: lấy các key đã có trong DB. Unscoped để tính cả bản đã xoá mềm: user xoá thông báo rồi
	// thì outbox gửi lại cùng event cũng không được tạo lại.
	var existing []string
	if err := r.db.WithContext(ctx).Unscoped().Model(&model.Notification{}).
		Where("idempotency_key IN ?", keys).Pluck("idempotency_key", &existing).Error; err != nil {
		return nil, err
	}

	// Bước 3: chuyển danh sách key đã có thành map để tra O(1) khi lọc.
	skip := make(map[string]bool, len(existing))
	for _, key := range existing {
		skip[key] = true
	}

	// Bước 4: chỉ giữ các thông báo có key chưa tồn tại. Giữ nguyên con trỏ của lô gốc nên ID được gán
	// sau khi insert cũng thấy được ở phía service.
	fresh := make([]*model.Notification, 0, len(notifications))
	for _, notification := range notifications {
		if !skip[notification.IdempotencyKey] {
			fresh = append(fresh, notification)
		}
	}

	// Cả lô đều đã có (outbox gửi lại event đã xử lý): không insert, trả rỗng để service không
	// broadcast hay push lại lần nữa.
	if len(fresh) == 0 {
		return nil, nil
	}

	// Bước 5: insert một lần cho cả lô (một câu INSERT nhiều VALUES, RETURNING id).
	// ON CONFLICT vẫn giữ để không vỡ unique khi hai lần gọi cùng event chạy song song và cùng lọt qua bước 2.
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "idempotency_key"}}, DoNothing: true}).
		Create(&fresh)
	if result.Error != nil {
		return nil, result.Error
	}

	// Bước 6: số dòng thêm được phải bằng số bản cần thêm. Thiếu dòng là đã có lần gọi khác chen vào giữa
	// bước 2 và bước 5, ID trong fresh có thể đã lệch: trả lỗi để transaction rollback, bên gọi (outbox)
	// thử lại và lần sau các key đó sẽ bị lọc ra ở bước 2.
	if result.RowsAffected != int64(len(fresh)) {
		return nil, ErrConcurrentInsert
	}

	return fresh, nil
}
