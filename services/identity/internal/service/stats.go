package service

import (
	"context"
	"net/http"

	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
)

// **Reviewed**
func (s *service) GetAdminStats(ctx context.Context, query dto.AdminStatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError) {
	resp, err := s.repo.GetAdminStats(ctx, query.Range())
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get user stats", err)
	}
	return resp, nil
}
