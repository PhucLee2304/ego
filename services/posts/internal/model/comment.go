package model

import "gorm.io/gorm"

type PostComment struct {
	gorm.Model
	PostID        uint    `gorm:"not null;index"`
	UserID        string  `gorm:"type:text;not null;index"`
	Content       string  `gorm:"type:text;not null"`
	ParentID      *uint   `gorm:"index"`
	ReplyToUserID *string `gorm:"type:text"`
}
