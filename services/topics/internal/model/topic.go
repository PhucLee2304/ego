package model

import "gorm.io/gorm"

type Topic struct {
	gorm.Model
	Name    string `gorm:"not null"`
	IsAudio bool   `gorm:"default:false"`
	Url     string `gorm:"not null"`
}
