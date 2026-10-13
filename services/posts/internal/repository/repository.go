package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/posts/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}

func (r *Repository) CreatePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *Repository) GetPost(ctx context.Context, id uint) (*model.Post, error) {
	var post model.Post
	err := r.db.WithContext(ctx).First(&post, id).Error
	return &post, err
}

func (r *Repository) GetPostForUpdate(ctx context.Context, id uint) (*model.Post, error) {
	var post model.Post
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&post, id).Error
	return &post, err
}

func (r *Repository) ListPosts(ctx context.Context, scope model.PostScope, classroomID *uint, status model.PostStatus, pinned *bool, pagination httpx.PaginationQuery) ([]*model.Post, int64, error) {
	var items []*model.Post
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Post{}).Where("scope = ?", scope)
	if classroomID != nil {
		db = db.Where("classroom_id = ?", *classroomID)
	} else {
		db = db.Where("classroom_id IS NULL")
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if pinned != nil {
		db = db.Where("pinned = ?", *pinned)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("created_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&items).Error
	return items, total, err
}

func (r *Repository) ListCommunityPostsByAuthor(ctx context.Context, authorID string, pagination httpx.PaginationQuery) ([]*model.Post, int64, error) {
	var posts []*model.Post
	var total int64
	db := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Where("scope = ? AND classroom_id IS NULL AND author_id = ?", model.PostScopeCommunity, authorID).
		Where("status IN ?", []model.PostStatus{model.PostStatusPublished, model.PostStatusHidden})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("created_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&posts).Error
	return posts, total, err
}

func (r *Repository) SavePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *Repository) DeletePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Delete(post).Error
}

func (r *Repository) CreateComment(ctx context.Context, comment *model.PostComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *Repository) GetComment(ctx context.Context, id uint) (*model.PostComment, error) {
	var comment model.PostComment
	err := r.db.WithContext(ctx).First(&comment, id).Error
	return &comment, err
}

func (r *Repository) ListComments(ctx context.Context, postID uint) ([]*model.PostComment, error) {
	var comments []*model.PostComment
	err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Order("created_at ASC, id ASC").
		Find(&comments).Error
	return comments, err
}

func (r *Repository) SaveComment(ctx context.Context, comment *model.PostComment) error {
	return r.db.WithContext(ctx).Save(comment).Error
}

func (r *Repository) DeleteComment(ctx context.Context, comment *model.PostComment) error {
	return r.db.WithContext(ctx).Delete(comment).Error
}

func (r *Repository) LikePost(ctx context.Context, postID uint, userID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.PostLike{PostID: postID, UserID: userID})
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) UnlikePost(ctx context.Context, postID uint, userID string) error {
	return r.db.WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.PostLike{}).Error
}

func (r *Repository) LikeComment(ctx context.Context, commentID uint, userID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.CommentLike{CommentID: commentID, UserID: userID})
	return result.RowsAffected == 1, result.Error
}

func (r *Repository) UnlikeComment(ctx context.Context, commentID uint, userID string) error {
	return r.db.WithContext(ctx).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Delete(&model.CommentLike{}).Error
}

func (r *Repository) PostStats(ctx context.Context, postID uint, userID string) (int64, int64, bool, error) {
	var likeCount int64
	var commentCount int64
	var currentUserLikeCount int64

	if err := r.db.WithContext(ctx).Model(&model.PostLike{}).Where("post_id = ?", postID).Count(&likeCount).Error; err != nil {
		return 0, 0, false, err
	}
	if err := r.db.WithContext(ctx).Model(&model.PostComment{}).Where("post_id = ?", postID).Count(&commentCount).Error; err != nil {
		return 0, 0, false, err
	}
	if err := r.db.WithContext(ctx).Model(&model.PostLike{}).Where("post_id = ? AND user_id = ?", postID, userID).Count(&currentUserLikeCount).Error; err != nil {
		return 0, 0, false, err
	}

	return likeCount, commentCount, currentUserLikeCount > 0, nil
}

func (r *Repository) CommentStats(ctx context.Context, commentID uint, userID string) (int64, bool, error) {
	var likeCount int64
	var currentUserLikeCount int64

	if err := r.db.WithContext(ctx).Model(&model.CommentLike{}).Where("comment_id = ?", commentID).Count(&likeCount).Error; err != nil {
		return 0, false, err
	}
	if err := r.db.WithContext(ctx).Model(&model.CommentLike{}).Where("comment_id = ? AND user_id = ?", commentID, userID).Count(&currentUserLikeCount).Error; err != nil {
		return 0, false, err
	}

	return likeCount, currentUserLikeCount > 0, nil
}

func (r *Repository) CreateAudit(ctx context.Context, audit *model.PostModerationAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *Repository) ListAudits(ctx context.Context, postID uint) ([]*model.PostModerationAudit, error) {
	var audits []*model.PostModerationAudit
	err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Order("created_at DESC, id DESC").
		Find(&audits).Error
	return audits, err
}

func (r *Repository) CreateOutbox(ctx context.Context, outbox *model.NotificationOutbox) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(outbox).Error
}

func (r *Repository) ClaimOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error) {
	var outboxes []*model.NotificationOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND next_attempt_at <= ?", now).
			Order("id").
			Limit(limit).
			Find(&outboxes).Error; err != nil {
			return err
		}
		if len(outboxes) == 0 {
			return nil
		}

		ids := make([]uint, len(outboxes))
		for index, outbox := range outboxes {
			ids[index] = outbox.ID
		}

		return tx.Model(&model.NotificationOutbox{}).
			Where("id IN ?", ids).
			Update("next_attempt_at", now.Add(time.Minute)).Error
	})
	return outboxes, err
}

func (r *Repository) MarkOutboxProcessed(ctx context.Context, id uint, processedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{"processed_at": processedAt, "last_error": nil}).Error
}

func (r *Repository) MarkOutboxFailed(ctx context.Context, id uint, attempts int, nextAttemptAt time.Time, message string) error {
	return r.db.WithContext(ctx).
		Model(&model.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":        attempts,
			"next_attempt_at": nextAttemptAt,
			"last_error":      message,
		}).Error
}
