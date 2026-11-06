package repository

import (
	"context"
	"strings"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
)

const statsTotalsSQL = `
SELECT
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to) AS sent,
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to AND read_at IS NOT NULL) AS read,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to) AS prev_sent,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to AND read_at IS NOT NULL) AS prev_read
FROM notifications WHERE deleted_at IS NULL`

const statsDailySQL = `
SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, COUNT(*) AS sent, COUNT(read_at) AS read
FROM notifications WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY date ORDER BY date`

const statsTypesSQL = `
SELECT COALESCE(data->>'type', '') AS type, COUNT(*) AS sent, COUNT(read_at) AS read
FROM notifications WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY type`

const statsDevicesSQL = `SELECT COUNT(*) AS devices, COUNT(DISTINCT user_id) AS device_users FROM device_tokens`

// StatsTypeCount là số thông báo trong kỳ theo data.type; service gom thành nhóm.
type StatsTypeCount struct {
	Type       string
	Sent, Read int64
}

// GetAdminStats chạy các câu đếm cho màn thống kê admin; Categories do service gom từ types.
func (r *Repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, []StatsTypeCount, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from": rng.From, "to": rng.To, "prev_from": rng.PrevFrom, "prev_to": rng.PrevTo, "tz": httpx.StatsTimeZone,
	}
	resp := &dto.AdminStatsResponse{}
	var totals struct {
		Sent, Read, PrevSent, PrevRead int64
	}
	var devices struct {
		Devices, DeviceUsers int64
	}
	var types []StatsTypeCount
	queries := []struct {
		sql string
		dst any
	}{
		{statsTotalsSQL, &totals},
		{statsDailySQL, &resp.Daily},
		{statsTypesSQL, &types},
		{statsDevicesSQL, &devices},
	}
	for _, q := range queries {
		// Raw báo lỗi khi truyền map cho câu không có tham số @.
		values := []any{args}
		if !strings.Contains(q.sql, "@") {
			values = nil
		}
		if err := db.Raw(q.sql, values...).Scan(q.dst).Error; err != nil {
			return nil, nil, err
		}
	}
	resp.Sent = httpx.PeriodCount{Current: totals.Sent, Previous: totals.PrevSent}
	resp.Read = httpx.PeriodCount{Current: totals.Read, Previous: totals.PrevRead}
	resp.Devices = devices.Devices
	resp.DeviceUsers = devices.DeviceUsers
	return resp, types, nil
}
