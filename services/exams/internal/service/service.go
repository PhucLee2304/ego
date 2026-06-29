package service

import (
	"context"
	"ego/platform/httpx"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"
)

type Service interface {
	GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.Exam, int, error)
}

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

func mapToDTO(exam *model.Exam) *dto.Exam {
	return &dto.Exam{
		ID:          exam.ID,
		Title:       exam.Title,
		Description: exam.Description,
		IsPublic:    exam.IsPublic,
		Type:        string(exam.Type),
		Year:        exam.Year,
		CreatedAt:   exam.CreatedAt,
		UpdatedAt:   exam.UpdatedAt,
	}
}

func (s *service) GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.Exam, int, error) {
	exams, total, err := s.repo.GetList(ctx, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, err
	}

	examDTOs := make([]*dto.Exam, len(exams))
	for i, exam := range exams {
		examDTOs[i] = mapToDTO(exam)
	}

	pageCounts := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if pageCounts == 0 {
		pageCounts = 1
	}

	return examDTOs, pageCounts, nil
}
