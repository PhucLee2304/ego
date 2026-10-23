package repository

import (
	"context"
	"strings"

	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
)

func (r *repository) CreateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Create(classroom).Error
}

func (r *repository) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Classroom{})
	search := strings.TrimSpace(query.Query)
	if search != "" {
		like := "%" + search + "%"
		if len(teacherIDs) > 0 {
			db = db.Where("name ILIKE ? OR description ILIKE ? OR teacher_id IN ?", like, like, teacherIDs)
		} else {
			db = db.Where("name ILIKE ? OR description ILIKE ?", like, like)
		}
	}
	if query.Active != nil {
		db = db.Where("active = ?", *query.Active)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("id ASC").
		Limit(int(query.Limit())).
		Offset(int(query.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *repository) GetClassroomsByUser(ctx context.Context, userID string, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	membership := r.db.Model(&model.Member{}).
		Select("1").
		Where("members.classroom_id = classrooms.id").
		Where("members.user_id = ? AND members.approved = ?", userID, true)
	db := r.db.WithContext(ctx).Model(&model.Classroom{}).
		Where("teacher_id = ? OR EXISTS (?)", userID, membership)
	search := strings.TrimSpace(query.Query)
	if search != "" {
		like := "%" + search + "%"
		if len(teacherIDs) > 0 {
			db = db.Where("name ILIKE ? OR description ILIKE ? OR teacher_id IN ?", like, like, teacherIDs)
		} else {
			db = db.Where("name ILIKE ? OR description ILIKE ?", like, like)
		}
	}
	if query.Active != nil {
		db = db.Where("active = ?", *query.Active)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("id ASC").
		Limit(int(query.Limit())).
		Offset(int(query.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *repository) GetClassroomByID(ctx context.Context, id uint) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).
		Preload("Members", "approved = ?", true).
		First(&classroom, id).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *repository) GetClassroomByCode(ctx context.Context, code string) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&classroom).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *repository) UpdateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Save(classroom).Error
}
