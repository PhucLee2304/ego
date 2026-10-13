package dto

import (
	"time"

	"ego/platform/httpx"
	"ego/services/posts/internal/model"
)

type PostsQuery struct {
	httpx.PaginationQuery
}

type ModerationPostsQuery struct {
	httpx.PaginationQuery
	Status model.PostStatus `schema:"status" validate:"omitempty,oneof=PUBLISHED HIDDEN"`
}

type CreatePostRequest struct {
	Content       string `json:"content" validate:"required,notblank"`
	AllowComments *bool  `json:"allowComments,omitempty"`
}

type UpdatePostRequest struct {
	Content       string `json:"content" validate:"required,notblank"`
	AllowComments *bool  `json:"allowComments,omitempty"`
}

type CreateCommentRequest struct {
	Content          string `json:"content" validate:"required,notblank"`
	ParentID         *uint  `json:"parentId,omitempty" validate:"omitempty,gt=0"`
	ReplyToCommentID *uint  `json:"replyToCommentId,omitempty" validate:"omitempty,gt=0"`
}

type UpdateCommentRequest struct {
	Content string `json:"content" validate:"required,notblank"`
}

type ModerationRequest struct {
	Reason string `json:"reason" validate:"required,notblank"`
}

type UserInfo struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Email  string  `json:"email"`
	Avatar *string `json:"avatar,omitempty"`
}

type PostResponse struct {
	ID            uint             `json:"id"`
	AuthorID      string           `json:"authorId"`
	Author        *UserInfo        `json:"author,omitempty"`
	Scope         model.PostScope  `json:"scope"`
	ClassroomID   *uint            `json:"classroomId,omitempty"`
	Content       string           `json:"content"`
	Status        model.PostStatus `json:"status"`
	Pinned        bool             `json:"pinned"`
	AllowComments bool             `json:"allowComments"`
	LikeCount     int64            `json:"likeCount"`
	CommentCount  int64            `json:"commentCount"`
	Liked         bool             `json:"liked"`
	CanEdit       bool             `json:"canEdit"`
	CanDelete     bool             `json:"canDelete"`
	CanPin        bool             `json:"canPin"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
}

type PostsResponse struct {
	Posts     []*PostResponse `json:"posts"`
	Page      int             `json:"page"`
	PageCount int             `json:"pageCount"`
}

type CommentResponse struct {
	ID            uint      `json:"id"`
	PostID        uint      `json:"postId"`
	UserID        string    `json:"userId"`
	User          *UserInfo `json:"user,omitempty"`
	Content       string    `json:"content"`
	ParentID      *uint     `json:"parentId,omitempty"`
	ReplyToUserID *string   `json:"replyToUserId,omitempty"`
	ReplyToUser   *UserInfo `json:"replyToUser,omitempty"`
	LikeCount     int64     `json:"likeCount"`
	Liked         bool      `json:"liked"`
	CanEdit       bool      `json:"canEdit"`
	CanDelete     bool      `json:"canDelete"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CommentsResponse struct {
	Comments []*CommentResponse `json:"comments"`
}

type AuditResponse struct {
	ID          uint                   `json:"id"`
	PostID      uint                   `json:"postId"`
	AdminUserID string                 `json:"adminUserId"`
	Action      model.ModerationAction `json:"action"`
	Reason      string                 `json:"reason"`
	CreatedAt   time.Time              `json:"createdAt"`
}
