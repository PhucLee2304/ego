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
	Content       string `json:"content" validate:"omitempty,notblank"`
	AllowComments *bool  `json:"allowComments,omitempty"`
	AttachmentIDs []uint `json:"attachmentIds,omitempty" validate:"dive,gt=0"`
}

func (request CreatePostRequest) CommentsAllowed() bool {
	return request.AllowComments == nil || *request.AllowComments
}

type UpdatePostRequest struct {
	Content       *string `json:"content,omitempty"`
	AllowComments *bool   `json:"allowComments,omitempty"`
	AttachmentIDs *[]uint `json:"attachmentIds,omitempty" validate:"omitempty,dive,gt=0"`
}

type CreateUploadIntentRequest struct {
	OriginalName string `json:"originalName" validate:"required,notblank"`
	ContentType  string `json:"contentType" validate:"required,notblank"`
	Size         int64  `json:"size" validate:"required,gt=0"`
	// ClassroomID có khi file đính kèm cho bài viết trong lớp (lưu ở posts/classrooms, chỉ giáo viên của lớp),
	// bỏ trống là bài viết cộng đồng (posts/community).
	ClassroomID *uint `json:"classroomId,omitempty" validate:"omitempty,gt=0"`
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

type CreateCommentRequest struct {
	Content          string `json:"content" validate:"required,notblank"`
	ParentID         *uint  `json:"parentId,omitempty" validate:"omitempty,gt=0"`
	ReplyToCommentID *uint  `json:"replyToCommentId,omitempty" validate:"omitempty,gt=0"`
}

type ModerationRequest struct {
	Reason string `json:"reason" validate:"required,notblank"`
}

type ModerationQuery struct {
	Reason string `schema:"reason" validate:"required,notblank"`
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

// PostStats là số like, số comment và trạng thái đã like của người xem cho một bài (repository đếm gộp).
type PostStats struct {
	PostID       uint
	LikeCount    int64
	CommentCount int64
	Liked        bool
}

// ToPostResponses ghép bài với số liệu, đính kèm và tác giả; bài thiếu số liệu thì tính là 0.
func ToPostResponses(posts []*model.Post, userID string, teacher bool, stats map[uint]*PostStats, attachments []*model.PostAttachment, users []*identityClient.UserInfo) []*PostResponse {
	attachmentsByPost := make(map[uint][]*AttachmentResponse, len(posts))
	for _, attachment := range attachments {
		attachmentsByPost[*attachment.PostID] = append(attachmentsByPost[*attachment.PostID], ToAttachmentResponse(attachment))
	}
	usersByID := ToUserInfoMap(users)
	responses := make([]*PostResponse, len(posts))
	for i, post := range posts {
		stat := stats[post.ID]
		if stat == nil {
			stat = &PostStats{}
		}
		postAttachments := attachmentsByPost[post.ID]
		if postAttachments == nil {
			postAttachments = []*AttachmentResponse{}
		}
		responses[i] = &PostResponse{
			ID:            post.ID,
			AuthorID:      post.AuthorID,
			Author:        usersByID[post.AuthorID],
			Scope:         post.Scope,
			ClassroomID:   post.ClassroomID,
			Content:       post.Content,
			Status:        post.Status,
			Pinned:        post.Pinned,
			AllowComments: post.AllowComments,
			LikeCount:     stat.LikeCount,
			CommentCount:  stat.CommentCount,
			Liked:         stat.Liked,
			CanEdit:       post.AuthorID == userID,
			CanDelete:     post.AuthorID == userID,
			CanPin:        teacher,
			CreatedAt:     post.CreatedAt,
			UpdatedAt:     post.UpdatedAt,
			Attachments:   postAttachments,
		}
	}
	return responses
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

// CommentStats là số like, số reply (chỉ comment gốc mới có) và trạng thái đã like của người xem (repository đếm gộp).
type CommentStats struct {
	CommentID  uint
	LikeCount  int64
	ReplyCount int64
	Liked      bool
}

// ToCommentResponses ghép comment với số liệu, người viết và người được reply; thiếu số liệu thì tính là 0.
func ToCommentResponses(comments []*model.PostComment, userID string, teacher bool, stats map[uint]*CommentStats, users []*identityClient.UserInfo) []*CommentResponse {
	usersByID := ToUserInfoMap(users)
	responses := make([]*CommentResponse, len(comments))
	for i, comment := range comments {
		stat := stats[comment.ID]
		if stat == nil {
			stat = &CommentStats{}
		}
		responses[i] = &CommentResponse{
			ID: comment.ID, PostID: comment.PostID, UserID: comment.UserID, User: usersByID[comment.UserID], Content: comment.Content,
			ParentID: comment.ParentID, ReplyToUserID: comment.ReplyToUserID,
			LikeCount: stat.LikeCount, ReplyCount: stat.ReplyCount, Liked: stat.Liked, CanEdit: comment.UserID == userID,
			CanDelete: comment.UserID == userID || teacher, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt,
		}
		if comment.ReplyToUserID != nil {
			responses[i].ReplyToUser = usersByID[*comment.ReplyToUserID]
		}
	}
	return responses
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
