package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"
)

func (r *repository) CreateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Create(schedule).Error
}

func (r *repository) GetScheduleByID(ctx context.Context, id uint) (*model.Schedule, error) {
	var schedule model.Schedule
	if err := r.db.WithContext(ctx).Preload("Room").Where("id = ?", id).First(&schedule).Error; err != nil {
		return nil, err
	}
	return &schedule, nil
}

func (r *repository) GetSchedulesByRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Schedule, error) {
	var schedules []*model.Schedule
	if err := r.db.WithContext(ctx).
		Preload("Room").
		Where("classroom_id = ? AND starts_at < ? AND COALESCE(ends_at, starts_at) >= ?", classroomID, to, from).
		Order("starts_at ASC, id ASC").
		Find(&schedules).Error; err != nil {
		return nil, err
	}
	return schedules, nil
}

func (r *repository) UpdateSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Save(schedule).Error
}

func (r *repository) DeleteSchedule(ctx context.Context, schedule *model.Schedule) error {
	return r.db.WithContext(ctx).Delete(schedule).Error
}
