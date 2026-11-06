package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
)

// GetAdminStats trả số liệu học liệu cho màn thống kê admin.
func (s *service) GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError) {
	resp, err := s.repo.GetAdminStats(ctx, query.Range())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load knowledge stats")
	}
	return resp, nil
}
