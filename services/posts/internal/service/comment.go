package service

import (
	"context"
	"fmt"

	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
)

func (s *service) ListPostComments(ctx context.Context, userID string, postID uint, query dto.CommentsQuery) (*dto.CommentsResponse, *httpx.HTTPError) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, ErrNotFound
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
		}
		teacher = access.IsTeacher
	}
	query.Normalize()
	items, total, err := s.repo.ListComments(ctx, postID, query.ParentID, query.Limit(), query.Offset())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
	}
	out, err := s.hydrateComments(ctx, items, userID, teacher)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
	}
	return &dto.CommentsResponse{
		Comments:  out,
		Page:      int(query.Page),
		PageCount: httpx.ToPageCounts(total, query.PageSize),
		Total:     total,
	}, nil
}

// GetPostComment trả một comment (kèm số reply nếu là comment gốc) để client
// mở thẳng tới comment được thông báo.
func (s *service) GetPostComment(ctx context.Context, userID string, id uint) (*dto.CommentResponse, *httpx.HTTPError) {
	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post comment")
	}
	p := c.Post
	if p == nil {
		return nil, ErrNotFound
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, ErrNotFound
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to get post comment")
		}
		teacher = access.IsTeacher
	}
	out, err := s.hydrateComments(ctx, []*model.PostComment{c}, userID, teacher)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post comment")
	}
	return out[0], nil
}

func (s *service) CreatePostComment(ctx context.Context, userID string, postID uint, req dto.CreateCommentRequest) (*dto.CommentResponse, *httpx.HTTPError) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, ErrNotFound
	}
	if !p.AllowComments {
		return nil, ErrCommentsDisabled
	}
	if p.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
		}
		if !access.Active {
			return nil, ErrClassroomReadOnly
		}
	}
	if req.ParentID != nil {
		return nil, ErrInvalidReply
	}
	c := &model.PostComment{
		PostID:  postID,
		UserID:  userID,
		Content: req.Content,
	}
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		recipient := p.AuthorID
		dataType := "POST_COMMENTED"
		body := "Someone commented on your post."
		if req.ReplyToCommentID != nil {
			target, err := repo.GetComment(ctx, *req.ReplyToCommentID)
			if err != nil || target.PostID != postID {
				return ErrInvalidReply
			}
			if target.ParentID == nil {
				c.ParentID = &target.ID
			} else {
				c.ParentID = target.ParentID
			}
			c.ReplyToUserID = &target.UserID
			recipient = target.UserID
			dataType = "COMMENT_REPLIED"
			body = "Someone replied to your comment."
		}
		if err := repo.CreateComment(ctx, c); err != nil {
			return err
		}
		if recipient == userID {
			return nil
		}
		return createInteractionOutbox(ctx, repo,
			fmt.Sprintf("comment:%d:created", c.ID), recipient,
			"New comment", body, dataType, p.ID, &c.ID, p.ClassroomID)
	}); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
	}
	out, err := s.hydrateComments(ctx, []*model.PostComment{c}, userID, false)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
	}
	return out[0], nil
}
