package repository

import (
	"context"

	"ego/services/posts/internal/model"
)

func (r *repository) CreateAudit(ctx context.Context, audit *model.PostModerationAudit) error {
	return r.db.WithContext(ctx).Create(audit).Error
}

func (r *repository) ListAudits(ctx context.Context, postID uint) ([]*model.PostModerationAudit, error) {
	var audits []*model.PostModerationAudit
	err := r.db.WithContext(ctx).
		Where("post_id = ?", postID).
		Order("created_at DESC, id DESC").
		Find(&audits).Error
	return audits, err
}
