package model

import "time"

type PostLike struct {
	ID        uint      `gorm:"primaryKey"`
	PostID    uint      `gorm:"not null;uniqueIndex:idx_post_likes_post_user"`
	UserID    string    `gorm:"type:text;not null;uniqueIndex:idx_post_likes_post_user"`
	CreatedAt time.Time `gorm:"not null"`
}

type CommentLike struct {
	ID        uint      `gorm:"primaryKey"`
	CommentID uint      `gorm:"not null;uniqueIndex:idx_comment_likes_comment_user"`
	UserID    string    `gorm:"type:text;not null;uniqueIndex:idx_comment_likes_comment_user"`
	CreatedAt time.Time `gorm:"not null"`
}
