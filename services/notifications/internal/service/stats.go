package service

import (
	"context"
	"sort"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
)

// categoryOther gom data.type không thuộc nhóm nào trong model.CategoryTypes.
const categoryOther = "OTHER"

// GetAdminStats trả số liệu thông báo cho màn thống kê admin, gom data.type theo nhóm.
func (s *service) GetAdminStats(ctx context.Context, query httpx.StatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError) {
	resp, types, err := s.repo.GetAdminStats(ctx, query.Range())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to load notification stats")
	}
	byCategory := map[string]*dto.CategoryStat{}
	for _, t := range types {
		category := model.CategoryOf(t.Type)
		if category == "" {
			category = categoryOther
		}
		stat, ok := byCategory[category]
		if !ok {
			stat = &dto.CategoryStat{Category: category}
			byCategory[category] = stat
		}
		stat.Sent += t.Sent
		stat.Read += t.Read
	}
	resp.Categories = make([]*dto.CategoryStat, 0, len(byCategory))
	for _, stat := range byCategory {
		resp.Categories = append(resp.Categories, stat)
	}
	sort.Slice(resp.Categories, func(i, j int) bool {
		a, b := resp.Categories[i], resp.Categories[j]
		if a.Sent != b.Sent {
			return a.Sent > b.Sent
		}
		return a.Category < b.Category
	})
	return resp, nil
}
