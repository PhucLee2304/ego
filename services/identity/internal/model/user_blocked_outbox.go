package model

import (
	"time"

	"gorm.io/gorm"
)

// UserBlockedOutbox là event "user bị khoá" chờ worker báo sang service khác: notifications xoá token push
// và đóng WebSocket thông báo, learning đưa user ra khỏi phòng live và đóng WebSocket bài thi/lớp học.
// Ghi cùng transaction với blocked_at nên không bị mất khi rpc lỗi hay identity restart giữa chừng.
type UserBlockedOutbox struct {
	gorm.Model
	UserID        string     `gorm:"type:text;not null;index"`
	Attempts      int        `gorm:"not null;default:0"`
	NextAttemptAt time.Time  `gorm:"not null;index"`
	ProcessedAt   *time.Time `gorm:"index"`
	LastError     *string    `gorm:"type:text"`
}
