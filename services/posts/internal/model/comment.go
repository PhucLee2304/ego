package model

import "gorm.io/gorm"

type PostComment struct {
	gorm.Model
	PostID   uint         `gorm:"not null;index"`
	Post     *Post        `gorm:"foreignKey:PostID;references:ID"`
	ParentID *uint        `gorm:"index"`
	Parent   *PostComment `gorm:"foreignKey:ParentID;references:ID"`

	UserID        string  `gorm:"type:text;not null;index"`
	ReplyToUserID *string `gorm:"type:text"`
	Content       string  `gorm:"type:text;not null"`
}
