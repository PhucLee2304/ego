package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
)

func (s *service) ListPostsForModeration(ctx context.Context, userID string, q dto.ModerationPostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, q.Status, nil, q.Limit(), q.Offset())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list posts for moderation")
	}
	out, err := s.hydratePosts(ctx, items, userID, false)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list posts for moderation")
	}
	return dto.ToPostsResponse(out, total, int(q.Page), int(q.PageSize)), nil
}

func (s *service) ModeratePost(ctx context.Context, adminID string, id uint, action model.ModerationAction, reason string) *httpx.HTTPError {
	return httpx.FromError(s.repo.Transaction(ctx, func(r repository.Repository) error {
		p, err := r.GetPostForUpdate(ctx, id)
		if err != nil {
			return err
		}
		switch action {
		case model.ModerationHide:
			p.Status = model.PostStatusHidden
		case model.ModerationUnhide:
			p.Status = model.PostStatusPublished
		case model.ModerationDelete:
			if err := r.CreateAudit(ctx, &model.PostModerationAudit{PostID: id, AdminUserID: adminID, Action: action, Reason: reason}); err != nil {
				return err
			}
			return r.DeletePost(ctx, p)
		}
		if err := r.SavePost(ctx, p); err != nil {
			return err
		}
		return r.CreateAudit(ctx, &model.PostModerationAudit{PostID: id, AdminUserID: adminID, Action: action, Reason: reason})
	}), "[ERROR] Failed to moderate post")
}

func (s *service) GetPostModerationAudits(ctx context.Context, id uint) ([]*dto.AuditResponse, *httpx.HTTPError) {
	items, err := s.repo.ListAudits(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post moderation audits")
	}
	return dto.ToAuditResponses(items), nil
}
