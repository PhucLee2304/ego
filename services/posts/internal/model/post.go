package model

import "gorm.io/gorm"

type PostScope string
type PostStatus string

const (
	PostScopeCommunity  PostScope  = "COMMUNITY"
	PostScopeClassroom  PostScope  = "CLASSROOM"
	PostStatusPublished PostStatus = "PUBLISHED"
	PostStatusHidden    PostStatus = "HIDDEN"
)

type Post struct {
	gorm.Model
	AuthorID      string     `gorm:"type:text;not null;index"`
	Scope         PostScope  `gorm:"type:text;not null;index"`
	ClassroomID   *uint      `gorm:"index"`
	Content       string     `gorm:"type:text;not null"`
	Status        PostStatus `gorm:"type:text;not null;default:PUBLISHED;index"`
	Pinned        bool       `gorm:"not null;default:false;index"`
	AllowComments bool       `gorm:"not null;default:true"`
}
