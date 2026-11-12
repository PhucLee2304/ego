package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	FirebaseUID string `gorm:"not null;uniqueIndex"`
	Email       string `gorm:"not null;index"`
	Name        string `gorm:"not null"`
	Avatar      *string
	Role        RoleName `gorm:"not null;default:CLIENT"`
	// BlockedAt khác nil là tài khoản đang bị admin khoá: không đăng nhập và không làm mới phiên được.
	BlockedAt *time.Time `gorm:"index"`
}

type RoleName string

const (
	RoleClient RoleName = "CLIENT"
	RoleAdmin  RoleName = "ADMIN"
)
