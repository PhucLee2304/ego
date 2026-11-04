package dto

import (
	"sort"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	postsConfig "ego/services/posts/config"
	"ego/services/posts/internal/model"
)

type PostsQuery struct {
	httpx.PaginationQuery
}

// CommentsQuery: không có parentId → trang comment gốc; có parentId → trang
// reply của comment gốc đó.
type CommentsQuery struct {
	httpx.PaginationQuery
	ParentID *uint `schema:"parentId" validate:"omitempty,gt=0"`
}

type ModerationPostsQuery struct {
	httpx.PaginationQuery
	Status model.PostStatus `schema:"status" validate:"omitempty,oneof=PUBLISHED HIDDEN"`
}

type CreatePostRequest struct {
	Content       string `json:"content"`
	AllowComments *bool  `json:"allowComments,omitempty"`
	AttachmentIDs []uint `json:"attachmentIds,omitempty" validate:"dive,gt=0"`
}

func (request CreatePostRequest) CommentsAllowed() bool {
	return request.AllowComments == nil || *request.AllowComments
}

type UpdatePostRequest struct {
	Content       string  `json:"content"`
	AllowComments *bool   `json:"allowComments,omitempty"`
	AttachmentIDs *[]uint `json:"attachmentIds,omitempty" validate:"omitempty,dive,gt=0"`
}

type CreateUploadIntentRequest struct {
	OriginalName string `json:"originalName" validate:"required,notblank"`
	ContentType  string `json:"contentType" validate:"required,notblank"`
	Size         int64  `json:"size" validate:"required,gt=0"`
}

type UploadLimitsResponse struct {
	ImageMaxBytes  int64    `json:"imageMaxBytes"`
	FileMaxBytes   int64    `json:"fileMaxBytes"`
	VideoMaxBytes  int64    `json:"videoMaxBytes"`
	PostMaxBytes   int64    `json:"postMaxBytes"`
	MaxAttachments int      `json:"maxAttachments"`
	ImageTypes     []string `json:"imageTypes"`
	FileTypes      []string `json:"fileTypes"`
	VideoTypes     []string `json:"videoTypes"`
}

func ToUploadLimitsResponse(cfg *postsConfig.AppConfig, imageTypes, fileTypes, videoTypes map[string]bool) *UploadLimitsResponse {
	return &UploadLimitsResponse{
		ImageMaxBytes:  cfg.ImageMaxBytes,
		FileMaxBytes:   cfg.FileMaxBytes,
		VideoMaxBytes:  cfg.VideoMaxBytes,
		PostMaxBytes:   cfg.PostMaxBytes,
		MaxAttachments: cfg.MaxAttachments,
		ImageTypes:     contentTypes(imageTypes),
		FileTypes:      contentTypes(fileTypes),
		VideoTypes:     contentTypes(videoTypes),
	}
}

func contentTypes(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type UploadIntentResponse struct {
	ID          uint      `json:"id"`
	UploadURL   string    `json:"uploadUrl"`
	ContentType string    `json:"contentType"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type AttachmentResponse struct {
	ID               uint                   `json:"id"`
	URL              string                 `json:"url"`
	MediaType        model.AttachmentType   `json:"mediaType"`
	OriginalName     string                 `json:"originalName"`
	ContentType      string                 `json:"contentType"`
	Size             int64                  `json:"size"`
	SortOrder        int                    `json:"sortOrder"`
	Width            *int                   `json:"width,omitempty"`
	Height           *int                   `json:"height,omitempty"`
	DurationSeconds  *float64               `json:"durationSeconds,omitempty"`
	ThumbnailURL     *string                `json:"thumbnailUrl,omitempty"`
	ProcessingStatus model.ProcessingStatus `json:"processingStatus"`
}

func ToAttachmentResponse(attachment *model.PostAttachment) *AttachmentResponse {
	return &AttachmentResponse{
		ID:           attachment.ID,
		URL:          attachment.URL,
		MediaType:    attachment.MediaType,
		OriginalName: attachment.OriginalName,
		ContentType:  attachment.ContentType,
		Size:         attachment.Size,
		SortOrder:    attachment.SortOrder,
		Width:        attachment.Width, Height: attachment.Height, DurationSeconds: attachment.DurationSeconds,
		ThumbnailURL: attachment.ThumbnailURL, ProcessingStatus: attachment.ProcessingStatus,
	}
}

func ToAttachmentResponses(attachments []*model.PostAttachment) []*AttachmentResponse {
	responses := make([]*AttachmentResponse, len(attachments))
	for index, attachment := range attachments {
		responses[index] = ToAttachmentResponse(attachment)
	}
	return responses
}

type CreateCommentRequest struct {
	Content          string `json:"content" validate:"required,notblank"`
	ParentID         *uint  `json:"parentId,omitempty" validate:"omitempty,gt=0"`
	ReplyToCommentID *uint  `json:"replyToCommentId,omitempty" validate:"omitempty,gt=0"`
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
	ID            uint                  `json:"id"`
	AuthorID      string                `json:"authorId"`
	Author        *UserInfo             `json:"author,omitempty"`
	Scope         model.PostScope       `json:"scope"`
	ClassroomID   *uint                 `json:"classroomId,omitempty"`
	Content       string                `json:"content"`
	Status        model.PostStatus      `json:"status"`
	Pinned        bool                  `json:"pinned"`
	AllowComments bool                  `json:"allowComments"`
	LikeCount     int64                 `json:"likeCount"`
	CommentCount  int64                 `json:"commentCount"`
	Liked         bool                  `json:"liked"`
	CanEdit       bool                  `json:"canEdit"`
	CanDelete     bool                  `json:"canDelete"`
	CanPin        bool                  `json:"canPin"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	Attachments   []*AttachmentResponse `json:"attachments"`
}

type PostsResponse struct {
	Posts     []*PostResponse `json:"posts"`
	Page      int             `json:"page"`
	PageCount int             `json:"pageCount"`
	Total     int64           `json:"total"`
}

func ToPostResponse(post *model.Post, userID string, teacher bool, likeCount, commentCount int64, liked bool, attachments []*AttachmentResponse) *PostResponse {
	return &PostResponse{
		ID: post.ID, AuthorID: post.AuthorID, Scope: post.Scope, ClassroomID: post.ClassroomID,
		Content: post.Content, Status: post.Status, Pinned: post.Pinned,
		AllowComments: post.AllowComments, LikeCount: likeCount, CommentCount: commentCount,
		Liked: liked, CanEdit: post.AuthorID == userID, CanDelete: post.AuthorID == userID,
		CanPin: teacher, CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt, Attachments: attachments,
	}
}

func ToPostsResponse(posts []*PostResponse, total int64, page, pageSize int) *PostsResponse {
	return &PostsResponse{Posts: posts, Page: page, PageCount: httpx.ToPageCounts(total, int32(pageSize)), Total: total}
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
	ReplyCount    int64     `json:"replyCount"`
	Liked         bool      `json:"liked"`
	CanEdit       bool      `json:"canEdit"`
	CanDelete     bool      `json:"canDelete"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func ToCommentResponse(comment *model.PostComment, userID string, teacher bool, likeCount int64, liked bool) *CommentResponse {
	return &CommentResponse{
		ID: comment.ID, PostID: comment.PostID, UserID: comment.UserID, Content: comment.Content,
		ParentID: comment.ParentID, ReplyToUserID: comment.ReplyToUserID,
		LikeCount: likeCount, Liked: liked, CanEdit: comment.UserID == userID,
		CanDelete: comment.UserID == userID || teacher, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt,
	}
}

type CommentsResponse struct {
	Comments  []*CommentResponse `json:"comments"`
	Page      int                `json:"page"`
	PageCount int                `json:"pageCount"`
	Total     int64              `json:"total"`
}

type AuditResponse struct {
	ID          uint                   `json:"id"`
	PostID      uint                   `json:"postId"`
	AdminUserID string                 `json:"adminUserId"`
	Action      model.ModerationAction `json:"action"`
	Reason      string                 `json:"reason"`
	CreatedAt   time.Time              `json:"createdAt"`
}

func ToAuditResponses(audits []*model.PostModerationAudit) []*AuditResponse {
	responses := make([]*AuditResponse, len(audits))
	for index, audit := range audits {
		responses[index] = &AuditResponse{ID: audit.ID, PostID: audit.PostID, AdminUserID: audit.AdminUserID, Action: audit.Action, Reason: audit.Reason, CreatedAt: audit.CreatedAt}
	}
	return responses
}

func ToUserInfoMap(users []*identityClient.UserInfo) map[string]*UserInfo {
	result := make(map[string]*UserInfo, len(users))
	for _, user := range users {
		var avatar *string
		if user.Avatar != "" {
			avatar = &user.Avatar
		}
		result[user.Id] = &UserInfo{ID: user.Id, Name: user.Name, Email: user.Email, Avatar: avatar}
	}
	return result
}
