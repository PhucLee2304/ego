package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
	"ego/services/knowledge/internal/repository"
)

// Service gom nghiệp vụ của mọi domain trong knowledge (topic, dictionary, grammar).
// Method của từng domain được nhóm bằng comment `// Domain` và cài đặt ở <domain>.go.
// Method phục vụ HTTP trả *httpx.HTTPError, method nội bộ trả error với sentinel.
type Service interface {
	// Topic (topic → section → lesson → transcript)
	GetTopics(ctx context.Context) (*dto.GetTopicsResponse, *httpx.HTTPError)
	GetSectionsByTopic(ctx context.Context, topicID uint) (*dto.GetSectionsResponse, *httpx.HTTPError)
	GetLessonsBySection(ctx context.Context, sectionID uint) (*dto.GetLessonsResponse, *httpx.HTTPError)
	GetLessonByID(ctx context.Context, id uint) (*dto.GetLessonByIDResponse, *httpx.HTTPError)
}

type service struct {
	repo repository.Repository
}

func New(repo repository.Repository) Service {
	return &service{repo: repo}
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}
