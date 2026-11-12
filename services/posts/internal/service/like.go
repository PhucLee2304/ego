package service

import (
	"context"
	"fmt"

	"ego/platform/httpx"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
)

func (s *service) LikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError {
	return s.setPostLiked(ctx, userID, postID, true)
}

func (s *service) UnlikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError {
	return s.setPostLiked(ctx, userID, postID, false)
}

func (s *service) LikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError {
	return s.setCommentLiked(ctx, userID, commentID, true)
}

func (s *service) UnlikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError {
	return s.setCommentLiked(ctx, userID, commentID, false)
}

func (s *service) setPostLiked(ctx context.Context, userID string, postID uint, liked bool) *httpx.HTTPError {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update post like")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return ErrNotFound
	}
	if p.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *p.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update post like")
		}
		if !access.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return httpx.FromError(s.repo.Transaction(ctx, func(repo repository.Repository) error {
			created, err := repo.LikePost(ctx, postID, userID)
			if err != nil || !created || p.AuthorID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("post:%d:liked-by:%s", p.ID, userID), p.AuthorID,
				"New like", "Someone liked your post.", "POST_LIKED", p.ID, nil, p.ClassroomID)
		}), "[ERROR] Failed to update post like")
	}
	return httpx.FromError(s.repo.UnlikePost(ctx, postID, userID), "[ERROR] Failed to update post like")
}

func (s *service) setCommentLiked(ctx context.Context, userID string, commentID uint, liked bool) *httpx.HTTPError {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update comment like")
	}
	p := c.Post
	if p == nil {
		return ErrNotFound
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return ErrNotFound
	}
	if p.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *p.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update comment like")
		}
		if !access.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return httpx.FromError(s.repo.Transaction(ctx, func(repo repository.Repository) error {
			created, err := repo.LikeComment(ctx, commentID, userID)
			if err != nil || !created || c.UserID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("comment:%d:liked-by:%s", c.ID, userID), c.UserID,
				"New like", "Someone liked your comment.", "COMMENT_LIKED", p.ID, &c.ID, p.ClassroomID)
		}), "[ERROR] Failed to update comment like")
	}
	return httpx.FromError(s.repo.UnlikeComment(ctx, commentID, userID), "[ERROR] Failed to update comment like")
}
