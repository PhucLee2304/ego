package repository

import (
	"context"

	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
)

const statsTotalsSQL = `
SELECT
	COUNT(*) AS total,
	COUNT(*) FILTER (WHERE role = @admin) AS admins,
	COUNT(*) FILTER (WHERE role <> @admin) AS clients,
	COUNT(*) FILTER (WHERE blocked_at IS NOT NULL) AS blocked,
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to) AS signups,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to) AS prev_signups,
	COUNT(*) FILTER (WHERE blocked_at >= @from AND blocked_at < @to) AS blocked_in_period,
	COUNT(*) FILTER (WHERE created_at < @from) AS users_before
FROM users WHERE deleted_at IS NULL`

const statsDailySQL = `
SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, COUNT(*) AS count
FROM users WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY date ORDER BY date`

const statsHeatmapSQL = `
SELECT EXTRACT(ISODOW FROM created_at AT TIME ZONE @tz)::int AS weekday,
	EXTRACT(HOUR FROM created_at AT TIME ZONE @tz)::int AS hour, COUNT(*) AS count
FROM users WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY weekday, hour ORDER BY weekday, hour`

// GetAdminStats chạy các câu đếm cho màn thống kê người dùng của admin.
func (r *repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from": rng.From, "to": rng.To, "prev_from": rng.PrevFrom, "prev_to": rng.PrevTo,
		"tz": httpx.StatsTimeZone, "admin": model.RoleAdmin,
	}
	resp := &dto.AdminStatsResponse{Daily: []httpx.DailyCount{}, Heatmap: []*dto.SignupHeatCell{}}
	var totals struct {
		Total, Admins, Clients, Blocked, Signups, PrevSignups, BlockedInPeriod, UsersBefore int64
	}
	for _, q := range []struct {
		sql string
		dst any
	}{
		{statsTotalsSQL, &totals},
		{statsDailySQL, &resp.Daily},
		{statsHeatmapSQL, &resp.Heatmap},
	} {
		if err := db.Raw(q.sql, args).Scan(q.dst).Error; err != nil {
			return nil, err
		}
	}
	resp.Total, resp.Admins, resp.Clients, resp.Blocked = totals.Total, totals.Admins, totals.Clients, totals.Blocked
	resp.Signups = httpx.PeriodCount{Current: totals.Signups, Previous: totals.PrevSignups}
	resp.BlockedInPeriod = totals.BlockedInPeriod
	resp.UsersBefore = totals.UsersBefore
	return resp, nil
}
