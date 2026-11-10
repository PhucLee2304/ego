package model

import "time"

type PostLike struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"type:text;not null;uniqueIndex:idx_post_likes_post_user"`
	CreatedAt time.Time `gorm:"not null"`

	PostID uint  `gorm:"not null;uniqueIndex:idx_post_likes_post_user"`
	Post   *Post `gorm:"foreignKey:PostID;references:ID"`
}

type CommentLike struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    string    `gorm:"type:text;not null;uniqueIndex:idx_comment_likes_comment_user"`
	CreatedAt time.Time `gorm:"not null"`

	CommentID uint         `gorm:"not null;uniqueIndex:idx_comment_likes_comment_user"`
	Comment   *PostComment `gorm:"foreignKey:CommentID;references:ID"`
}
