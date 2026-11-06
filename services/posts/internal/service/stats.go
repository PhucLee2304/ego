package service

import (
	"context"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
)

// GetAdminStats trả số liệu cộng đồng cho màn thống kê admin, kèm tên top tác giả.
func (s *Service) GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError) {
	resp, err := s.repo.GetAdminStats(ctx, query.Range())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load post stats")
	}
	if len(resp.TopAuthors) == 0 {
		return resp, nil
	}
	ids := make([]string, len(resp.TopAuthors))
	for i, author := range resp.TopAuthors {
		ids[i] = author.UserID
	}
	usersResp, err := s.identity.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: ids})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load post stats")
	}
	users := dto.ToUserInfoMap(usersResp.Users)
	for _, author := range resp.TopAuthors {
		author.User = users[author.UserID]
	}
	return resp, nil
}
