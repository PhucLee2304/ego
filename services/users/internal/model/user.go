package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Email  string `gorm:"not null;unique"`
	Name   string `gorm:"not null"`
	Avatar *string
	Role   RoleName `gorm:"not null;default:CLIENT"`
}

type RoleName string

const (
	RoleClient RoleName = "CLIENT"
	RoleAdmin  RoleName = "ADMIN"
)
