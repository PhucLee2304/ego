package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	// Post
	CreatePost(ctx context.Context, post *model.Post) error
	GetPost(ctx context.Context, id uint) (*model.Post, error)
	GetPostForUpdate(ctx context.Context, id uint) (*model.Post, error)
	ListPosts(ctx context.Context, scope model.PostScope, classroomID *uint, status model.PostStatus, pinned *bool, limit, offset int32) ([]*model.Post, int64, error)
	ListCommunityPostsByAuthor(ctx context.Context, authorID string, limit, offset int32) ([]*model.Post, int64, error)
	SavePost(ctx context.Context, post *model.Post) error
	DeletePost(ctx context.Context, post *model.Post) error
	GetPostStats(ctx context.Context, postIDs []uint, userID string) (map[uint]*dto.PostStats, error)

	// Attachment
	CreateUploadIntent(ctx context.Context, intent *model.PostUploadIntent) error
	GetUploadIntent(ctx context.Context, id uint) (*model.PostUploadIntent, error)
	GetUploadIntentForUpdate(ctx context.Context, id uint) (*model.PostUploadIntent, error)
	SaveUploadIntent(ctx context.Context, intent *model.PostUploadIntent) error
	ListExpiredUploadIntents(ctx context.Context, now time.Time, limit int) ([]*model.PostUploadIntent, error)
	CancelUploadIntents(ctx context.Context, ids []uint) (int64, error)
	CreateAttachment(ctx context.Context, attachment *model.PostAttachment) error
	GetAttachment(ctx context.Context, id uint) (*model.PostAttachment, error)
	AttachToPost(ctx context.Context, ids []uint, userID string, postID uint) error
	DetachFromPost(ctx context.Context, postID uint, ids []uint) error
	ListAttachments(ctx context.Context, postIDs []uint) ([]*model.PostAttachment, error)
	ListOrphanAttachments(ctx context.Context, before time.Time, limit int) ([]*model.PostAttachment, error)
	DeleteOrphanAttachmentsPermanently(ctx context.Context, ids []uint) (int64, error)
	ListVideoAttachmentsForProcessing(ctx context.Context, limit int) ([]*model.PostAttachment, error)
	SaveAttachment(ctx context.Context, attachment *model.PostAttachment) error

	// Comment
	CreateComment(ctx context.Context, comment *model.PostComment) error
	GetComment(ctx context.Context, id uint) (*model.PostComment, error)
	ListComments(ctx context.Context, postID uint, parentID *uint, limit, offset int32) ([]*model.PostComment, int64, error)
	GetCommentStats(ctx context.Context, commentIDs []uint, userID string) (map[uint]*dto.CommentStats, error)

	// Like
	LikePost(ctx context.Context, postID uint, userID string) (bool, error)
	UnlikePost(ctx context.Context, postID uint, userID string) error
	LikeComment(ctx context.Context, commentID uint, userID string) (bool, error)
	UnlikeComment(ctx context.Context, commentID uint, userID string) error

	// Moderation
	CreateAudit(ctx context.Context, audit *model.PostModerationAudit) error
	ListAudits(ctx context.Context, postID uint) ([]*model.PostModerationAudit, error)

	// Notification outbox
	CreateOutbox(ctx context.Context, outbox *model.NotificationOutbox) error
	ClaimOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error)
	MarkOutboxProcessed(ctx context.Context, id uint, processedAt time.Time) error
	MarkOutboxFailed(ctx context.Context, id uint, attempts int, nextAttemptAt time.Time, message string) error

	// Stats (admin)
	GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error)

	Transaction(ctx context.Context, fn func(Repository) error) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}
