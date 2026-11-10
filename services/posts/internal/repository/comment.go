package repository

import (
	"context"

	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
)

const commentStatsSQL = `
SELECT c.id AS comment_id,
	(SELECT COUNT(*) FROM comment_likes l WHERE l.comment_id = c.id) AS like_count,
	(SELECT COUNT(*) FROM post_comments r WHERE r.parent_id = c.id AND r.deleted_at IS NULL) AS reply_count,
	EXISTS (SELECT 1 FROM comment_likes l WHERE l.comment_id = c.id AND l.user_id = ?) AS liked
FROM post_comments c WHERE c.id IN ?`

func (r *repository) CreateComment(ctx context.Context, comment *model.PostComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

// GetComment lấy comment kèm bài viết (Post nil nếu bài đã bị xóa).
func (r *repository) GetComment(ctx context.Context, id uint) (*model.PostComment, error) {
	var comment model.PostComment
	err := r.db.WithContext(ctx).Preload("Post").First(&comment, id).Error
	return &comment, err
}

// ListComments trả một trang comment gốc (parentID nil) hoặc reply của một comment gốc.
func (r *repository) ListComments(ctx context.Context, postID uint, parentID *uint, limit, offset int32) ([]*model.PostComment, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.PostComment{}).Where("post_id = ?", postID)
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", *parentID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var comments []*model.PostComment
	err := q.Order("created_at ASC, id ASC").Limit(int(limit)).Offset(int(offset)).Find(&comments).Error
	return comments, total, err
}

// GetCommentStats đếm like/reply của nhiều comment trong một câu truy vấn, key là comment ID.
func (r *repository) GetCommentStats(ctx context.Context, commentIDs []uint, userID string) (map[uint]*dto.CommentStats, error) {
	out := make(map[uint]*dto.CommentStats, len(commentIDs))
	if len(commentIDs) == 0 {
		return out, nil
	}
	var rows []*dto.CommentStats
	if err := r.db.WithContext(ctx).Raw(commentStatsSQL, userID, commentIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.CommentID] = row
	}
	return out, nil
}
