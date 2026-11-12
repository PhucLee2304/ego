package repository

import (
	"context"

	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"

	"gorm.io/gorm/clause"
)

const postStatsSQL = `
SELECT p.id AS post_id,
	(SELECT COUNT(*) FROM post_likes l WHERE l.post_id = p.id) AS like_count,
	(SELECT COUNT(*) FROM post_comments c WHERE c.post_id = p.id AND c.deleted_at IS NULL) AS comment_count,
	EXISTS (SELECT 1 FROM post_likes l WHERE l.post_id = p.id AND l.user_id = ?) AS liked
FROM posts p WHERE p.id IN ?`

func (r *repository) CreatePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *repository) GetPost(ctx context.Context, id uint) (*model.Post, error) {
	var post model.Post
	err := r.db.WithContext(ctx).First(&post, id).Error
	return &post, err
}

func (r *repository) GetPostForUpdate(ctx context.Context, id uint) (*model.Post, error) {
	var post model.Post
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&post, id).Error
	return &post, err
}

func (r *repository) ListPosts(ctx context.Context, scope model.PostScope, classroomID *uint, status model.PostStatus, pinned *bool, limit, offset int32) ([]*model.Post, int64, error) {
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
	err := db.Order("created_at DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&items).Error
	return items, total, err
}

func (r *repository) ListCommunityPostsByAuthor(ctx context.Context, authorID string, limit, offset int32) ([]*model.Post, int64, error) {
	var posts []*model.Post
	var total int64
	db := r.db.WithContext(ctx).
		Model(&model.Post{}).
		Where("scope = ? AND classroom_id IS NULL AND author_id = ?", model.PostScopeCommunity, authorID).
		Where("status IN ?", []model.PostStatus{model.PostStatusPublished, model.PostStatusHidden})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := db.Order("created_at DESC, id DESC").Limit(int(limit)).Offset(int(offset)).Find(&posts).Error
	return posts, total, err
}

func (r *repository) SavePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Save(post).Error
}

func (r *repository) DeletePost(ctx context.Context, post *model.Post) error {
	return r.db.WithContext(ctx).Delete(post).Error
}

// GetPostStats đếm like/comment của nhiều bài trong một câu truy vấn, key là post ID.
func (r *repository) GetPostStats(ctx context.Context, postIDs []uint, userID string) (map[uint]*dto.PostStats, error) {
	out := make(map[uint]*dto.PostStats, len(postIDs))
	if len(postIDs) == 0 {
		return out, nil
	}
	var rows []*dto.PostStats
	if err := r.db.WithContext(ctx).Raw(postStatsSQL, userID, postIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PostID] = row
	}
	return out, nil
}
