package service

import (
	"context"
	"ego/services/topics/internal/dto"
	"ego/services/topics/internal/repository"
)

type Service interface {
	GetTopics(ctx context.Context) (*dto.GetTopicsResponse, error)
	GetSectionsByTopic(ctx context.Context, topicID uint) (*dto.GetSectionsResponse, error)
	GetLessonsBySection(ctx context.Context, sectionID uint) (*dto.GetLessonsResponse, error)
	GetLessonByID(ctx context.Context, id uint) (*dto.GetLessonByIDResponse, error)
}

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetTopics(ctx context.Context) (*dto.GetTopicsResponse, error) {
	topics, err := s.repo.GetTopics(ctx)
	if err != nil {
		return nil, err
	}

	topicDTOs := make([]*dto.Topic, len(topics))
	for i, topic := range topics {
		topicDTOs[i] = &dto.Topic{
			ID:      topic.ID,
			Name:    topic.Name,
			IsAudio: topic.IsAudio,
			Url:     topic.Url,
		}
	}

	return &dto.GetTopicsResponse{Topics: topicDTOs}, nil
}

func (s *service) GetSectionsByTopic(ctx context.Context, topicID uint) (*dto.GetSectionsResponse, error) {
	sections, err := s.repo.GetSectionsByTopic(ctx, topicID)
	if err != nil {
		return nil, err
	}

	sectionDTOs := make([]*dto.Section, len(sections))
	for i, section := range sections {
		sectionDTOs[i] = &dto.Section{
			ID:      section.ID,
			Name:    section.Name,
			TopicID: section.TopicID,
		}
	}

	return &dto.GetSectionsResponse{Sections: sectionDTOs}, nil
}

func (s *service) GetLessonsBySection(ctx context.Context, sectionID uint) (*dto.GetLessonsResponse, error) {
	lessons, err := s.repo.GetLessonsBySection(ctx, sectionID)
	if err != nil {
		return nil, err
	}

	lessonDTOs := make([]*dto.Lesson, len(lessons))
	for i, lesson := range lessons {
		lessonDTOs[i] = &dto.Lesson{
			ID:          lesson.ID,
			Title:       lesson.Title,
			Description: lesson.Description,
			Subtitle:    lesson.Subtitle,
			Url:         lesson.Url,
			SectionID:   lesson.SectionID,
			Transcripts: []*dto.Transcript{},
		}
	}

	return &dto.GetLessonsResponse{Lessons: lessonDTOs}, nil
}

func (s *service) GetLessonByID(ctx context.Context, id uint) (*dto.GetLessonByIDResponse, error) {
	lesson, err := s.repo.GetLessonByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var transcriptDTOs []*dto.Transcript
	for _, t := range lesson.Transcripts {
		transcriptDTOs = append(transcriptDTOs, &dto.Transcript{
			ID:        t.ID,
			Content:   t.Content,
			Order:     t.Order,
			TimeStart: t.TimeStart,
			TimeEnd:   t.TimeEnd,
			Url:       t.Url,
			LessonID:  t.LessonID,
		})
	}

	lessonDTO := &dto.Lesson{
		ID:          lesson.ID,
		Title:       lesson.Title,
		Description: lesson.Description,
		Subtitle:    lesson.Subtitle,
		Url:         lesson.Url,
		SectionID:   lesson.SectionID,
		Transcripts: transcriptDTOs,
	}

	return &dto.GetLessonByIDResponse{Lesson: lessonDTO}, nil
}
