package repository

import (
	"context"
	"ego/services/topics/internal/model"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetTopics(ctx context.Context) ([]*model.Topic, error) {
	var topics []*model.Topic
	if err := r.db.WithContext(ctx).Find(&topics).Error; err != nil {
		return nil, err
	}
	return topics, nil
}

func (r *Repository) GetSectionsByTopic(ctx context.Context, topicID uint) ([]*model.Section, error) {
	var sections []*model.Section
	if err := r.db.WithContext(ctx).Where("topic_id = ?", topicID).Find(&sections).Error; err != nil {
		return nil, err
	}
	return sections, nil
}

func (r *Repository) GetLessonsBySection(ctx context.Context, sectionID uint) ([]*model.Lesson, error) {
	var lessons []*model.Lesson
	if err := r.db.WithContext(ctx).Where("section_id = ?", sectionID).Find(&lessons).Error; err != nil {
		return nil, err
	}
	return lessons, nil
}

func (r *Repository) GetLessonByID(ctx context.Context, id uint) (*model.Lesson, error) {
	var lesson model.Lesson
	if err := r.db.WithContext(ctx).Preload("Transcripts").Where("id = ?", id).First(&lesson).Error; err != nil {
		return nil, err
	}
	return &lesson, nil
}
