package repository

import (
	"context"
	"errors"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

func (r *repository) CountApprovedMembers(ctx context.Context, classroomID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Joins("JOIN classrooms ON classrooms.id = members.classroom_id").
		Where("members.classroom_id = ? AND members.approved = ?", classroomID, true).
		Where("members.user_id <> classrooms.teacher_id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) UpsertMember(ctx context.Context, member *model.Member) error {
	var existing model.Member
	db := r.db.WithContext(ctx)
	err := db.Unscoped().Where("classroom_id = ? AND user_id = ?", member.ClassroomID, member.UserID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return db.Create(member).Error
	}
	if err != nil {
		return err
	}
	if err := db.Unscoped().Model(&existing).Updates(map[string]any{
		"approved":    member.Approved,
		"approved_at": member.ApprovedAt,
		"joined_at":   member.JoinedAt,
		"deleted_at":  nil,
	}).Error; err != nil {
		return err
	}
	member.Model = existing.Model
	member.DeletedAt = gorm.DeletedAt{}
	return nil
}

func (r *repository) GetMember(ctx context.Context, classroomID uint, userID string) (*model.Member, error) {
	var member model.Member
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("classroom_id = ? AND user_id = ?", classroomID, userID).
		First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *repository) GetJoinRequests(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND approved IS NULL", classroomID).
		Preload("Classroom").
		Order("updated_at DESC, id DESC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *repository) DeleteMember(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).Delete(member).Error
}

func (r *repository) IsStudentApprovedInClassroom(ctx context.Context, classroomID uint, userID string) (bool, error) {
	var count int64
	approved := true
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND user_id = ? AND approved = ?", classroomID, userID, approved).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *repository) GetApprovedMembersByClassroom(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Where("classroom_id = ? AND approved = ?", classroomID, true).
		Order("joined_at ASC, id ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}
