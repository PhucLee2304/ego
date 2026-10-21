package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	FirebaseUID string `gorm:"not null;uniqueIndex"`
	Email       string `gorm:"not null;index"`
	Name        string `gorm:"not null"`
	Avatar      *string
	Role        RoleName `gorm:"not null;default:CLIENT"`
}

type RoleName string

const (
	RoleClient RoleName = "CLIENT"
	RoleAdmin  RoleName = "ADMIN"
)
