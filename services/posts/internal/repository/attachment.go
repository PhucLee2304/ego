package repository

import (
	"context"
	"time"

	"ego/services/posts/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *repository) CreateUploadIntent(ctx context.Context, intent *model.PostUploadIntent) error {
	return r.db.WithContext(ctx).Create(intent).Error
}

func (r *repository) GetUploadIntent(ctx context.Context, id uint) (*model.PostUploadIntent, error) {
	var intent model.PostUploadIntent
	err := r.db.WithContext(ctx).Preload("Attachment").First(&intent, id).Error
	return &intent, err
}

func (r *repository) GetUploadIntentForUpdate(ctx context.Context, id uint) (*model.PostUploadIntent, error) {
	var intent model.PostUploadIntent
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&intent, id).Error
	return &intent, err
}

func (r *repository) SaveUploadIntent(ctx context.Context, intent *model.PostUploadIntent) error {
	return r.db.WithContext(ctx).Save(intent).Error
}

func (r *repository) CancelUploadIntents(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Model(&model.PostUploadIntent{}).
		Where("id IN ? AND status = ?", ids, model.UploadStatusPending).
		Update("status", model.UploadStatusCancelled)
	return result.RowsAffected, result.Error
}

func (r *repository) ListExpiredUploadIntents(ctx context.Context, now time.Time, limit int) ([]*model.PostUploadIntent, error) {
	var intents []*model.PostUploadIntent
	err := r.db.WithContext(ctx).Where("status = ? AND expires_at < ?", model.UploadStatusPending, now).Limit(limit).Find(&intents).Error
	return intents, err
}

func (r *repository) CreateAttachment(ctx context.Context, attachment *model.PostAttachment) error {
	return r.db.WithContext(ctx).Create(attachment).Error
}

func (r *repository) GetAttachment(ctx context.Context, id uint) (*model.PostAttachment, error) {
	var attachment model.PostAttachment
	err := r.db.WithContext(ctx).First(&attachment, id).Error
	return &attachment, err
}

func (r *repository) AttachToPost(ctx context.Context, ids []uint, userID string, postID uint) error {
	if len(ids) == 0 {
		return nil
	}
	result := r.db.WithContext(ctx).Model(&model.PostAttachment{}).
		Where("id IN ? AND user_id = ? AND post_id IS NULL", ids, userID).
		Updates(map[string]any{"post_id": postID})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != int64(len(ids)) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DetachFromPost gỡ đính kèm khỏi bài; chúng thành orphan và job dọn dẹp sẽ xóa file sau.
func (r *repository) DetachFromPost(ctx context.Context, postID uint, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&model.PostAttachment{}).
		Where("post_id = ? AND id IN ?", postID, ids).
		Updates(map[string]any{"post_id": nil}).Error
}

// ListAttachments trả đính kèm của nhiều bài, sắp theo bài rồi theo thứ tự hiển thị.
func (r *repository) ListAttachments(ctx context.Context, postIDs []uint) ([]*model.PostAttachment, error) {
	var attachments []*model.PostAttachment
	if len(postIDs) == 0 {
		return attachments, nil
	}
	err := r.db.WithContext(ctx).Where("post_id IN ?", postIDs).Order("post_id ASC, sort_order ASC, id ASC").Find(&attachments).Error
	return attachments, err
}

func (r *repository) ListOrphanAttachments(ctx context.Context, before time.Time, limit int) ([]*model.PostAttachment, error) {
	var attachments []*model.PostAttachment
	err := r.db.WithContext(ctx).Where("post_id IS NULL AND created_at < ?", before).Limit(limit).Find(&attachments).Error
	return attachments, err
}

// DeleteOrphanAttachmentsPermanently xóa hẳn các đính kèm trong ids còn chưa gắn bài (post_id NULL); trả số dòng đã xóa.
func (r *repository) DeleteOrphanAttachmentsPermanently(ctx context.Context, ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := r.db.WithContext(ctx).Unscoped().
		Where("id IN ? AND post_id IS NULL", ids).
		Delete(&model.PostAttachment{})
	return result.RowsAffected, result.Error
}

func (r *repository) ListVideoAttachmentsForProcessing(ctx context.Context, limit int) ([]*model.PostAttachment, error) {
	var attachments []*model.PostAttachment
	err := r.db.WithContext(ctx).
		Where("media_type = ? AND processing_status = ? AND processing_attempts < 3", model.AttachmentTypeVideo, model.ProcessingStatusProcessing).
		Order("id ASC").
		Limit(limit).
		Find(&attachments).Error
	return attachments, err
}

func (r *repository) SaveAttachment(ctx context.Context, attachment *model.PostAttachment) error {
	return r.db.WithContext(ctx).Save(attachment).Error
}
