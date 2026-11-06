package service

import (
	"context"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

// GetAdminStats trả số liệu learning cho màn thống kê admin, kèm tên giáo viên của từng lớp.
func (s *service) GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError) {
	resp, teacherIDs, err := s.repo.GetAdminStats(ctx, query.Range())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load learning stats")
	}
	if len(teacherIDs) == 0 {
		return resp, nil
	}
	usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: teacherIDs})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load learning stats")
	}
	users := dto.ToUserInfoMap(usersResp.Users)
	for i, row := range resp.List {
		row.Teacher = users[teacherIDs[i]]
	}
	return resp, nil
}
