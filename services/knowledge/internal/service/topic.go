package service

import (
	"context"
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"

	"gorm.io/gorm"
)

func (s *service) GetTopics(ctx context.Context) (*dto.GetTopicsResponse, *httpx.HTTPError) {
	topics, err := s.repo.GetTopics(ctx)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get topics", err)
	}

	topicDTOs := make([]*dto.Topic, len(topics))
	for i, topic := range topics {
		topicDTOs[i] = dto.ToTopic(topic)
	}

	return &dto.GetTopicsResponse{Topics: topicDTOs}, nil
}

func (s *service) GetSectionsByTopic(ctx context.Context, topicID uint) (*dto.GetSectionsResponse, *httpx.HTTPError) {
	if _, err := s.repo.GetTopicByID(ctx, topicID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Topic not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get topic", err)
	}

	sections, err := s.repo.GetSectionsByTopic(ctx, topicID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get sections", err)
	}

	sectionDTOs := make([]*dto.Section, len(sections))
	for i, section := range sections {
		sectionDTOs[i] = dto.ToSection(section)
	}

	return &dto.GetSectionsResponse{Sections: sectionDTOs}, nil
}

func (s *service) GetLessonsBySection(ctx context.Context, sectionID uint) (*dto.GetLessonsResponse, *httpx.HTTPError) {
	lessons, err := s.repo.GetLessonsBySection(ctx, sectionID)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get lessons", err)
	}

	lessonDTOs := make([]*dto.Lesson, len(lessons))
	for i, lesson := range lessons {
		lessonDTOs[i] = dto.ToLesson(lesson)
	}

	return &dto.GetLessonsResponse{Lessons: lessonDTOs}, nil
}

func (s *service) GetLessonByID(ctx context.Context, id uint) (*dto.GetLessonByIDResponse, *httpx.HTTPError) {
	lesson, err := s.repo.GetLessonByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Lesson not found", err)
		}
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get lesson", err)
	}

	return &dto.GetLessonByIDResponse{Lesson: dto.ToLesson(lesson)}, nil
}
