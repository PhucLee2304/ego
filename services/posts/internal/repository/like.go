package repository

import (
	"context"

	"ego/services/posts/internal/model"

	"gorm.io/gorm/clause"
)

func (r *repository) LikePost(ctx context.Context, postID uint, userID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.PostLike{PostID: postID, UserID: userID})
	return result.RowsAffected == 1, result.Error
}

func (r *repository) UnlikePost(ctx context.Context, postID uint, userID string) error {
	return r.db.WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.PostLike{}).Error
}

func (r *repository) LikeComment(ctx context.Context, commentID uint, userID string) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&model.CommentLike{CommentID: commentID, UserID: userID})
	return result.RowsAffected == 1, result.Error
}

func (r *repository) UnlikeComment(ctx context.Context, commentID uint, userID string) error {
	return r.db.WithContext(ctx).
		Where("comment_id = ? AND user_id = ?", commentID, userID).
		Delete(&model.CommentLike{}).Error
}
