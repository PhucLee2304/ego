package service

import (
	"context"
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

func (s *service) GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int64, *httpx.HTTPError) {
	exams, total, err := s.repo.GetListExam(ctx, query.Type, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get exams", err)
	}

	examDTOs := make([]*dto.GetExamsResponse, len(exams))
	for i, exam := range exams {
		examDTOs[i] = dto.ToGetExamsResponse(exam)
	}

	return examDTOs, total, nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, *httpx.HTTPError) {
	exam, err := s.repo.GetExamByID(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get exam")
	}

	return dto.ToGetExamResponse(exam), nil
}
