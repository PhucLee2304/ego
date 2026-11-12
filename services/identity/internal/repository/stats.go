package repository

import (
	"context"

	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
)

// statsTotalsSQL đếm mọi chỉ số tổng trong một lần quét bảng users (bỏ qua bản ghi đã xoá mềm).
//   - total, admins, clients, blocked: trạng thái hiện tại, không phụ thuộc khoảng thời gian;
//     clients là mọi tài khoản không phải ADMIN.
//   - signups: số đăng ký mới trong kỳ [from, to); prev_signups: cùng chỉ số ở kỳ liền trước
//     [prev_from, prev_to) để so sánh tăng giảm.
//   - blocked_in_period: số tài khoản bị khoá trong kỳ, tính theo blocked_at.
//   - users_before: số tài khoản đã có trước đầu kỳ, làm mốc để cộng dồn biểu đồ tăng trưởng.
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

// statsDailySQL đếm số đăng ký mới theo từng ngày trong kỳ [from, to). Ngày được tính theo múi giờ
// @tz (không phải UTC) và trả về dạng chuỗi YYYY-MM-DD. Ngày không có đăng ký nào sẽ không có dòng.
const statsDailySQL = `
SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, COUNT(*) AS count
FROM users WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY date ORDER BY date`

// statsHeatmapSQL đếm số đăng ký mới trong kỳ [from, to) theo từng ô (thứ trong tuần, giờ trong ngày),
// tính theo múi giờ @tz, để vẽ heatmap. weekday theo chuẩn ISO: 1 là thứ Hai, 7 là Chủ nhật;
// hour từ 0 đến 23. Ô không có đăng ký nào sẽ không có dòng.
const statsHeatmapSQL = `
SELECT EXTRACT(ISODOW FROM created_at AT TIME ZONE @tz)::int AS weekday,
	EXTRACT(HOUR FROM created_at AT TIME ZONE @tz)::int AS hour, COUNT(*) AS count
FROM users WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY weekday, hour ORDER BY weekday, hour`

// GetAdminStats tổng hợp số liệu cho màn thống kê người dùng của admin trong khoảng rng.
// Ba câu SQL ở trên dùng chung một bộ tham số đặt tên (@from, @to, @prev_from, @prev_to, @tz,
// @admin) và chạy lần lượt. Câu nào lỗi thì trả lỗi ngay, các câu sau không chạy.
// Daily và Heatmap được khởi tạo bằng slice rỗng để JSON trả về [] thay vì null khi không có dữ liệu.
// **Reviewed**
func (r *repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from":      rng.From,
		"to":        rng.To,
		"prev_from": rng.PrevFrom,
		"prev_to":   rng.PrevTo,
		"tz":        httpx.StatsTimeZone,
		"admin":     model.RoleAdmin,
	}
	resp := &dto.AdminStatsResponse{
		Daily:   []httpx.DailyCount{},
		Heatmap: []*dto.SignupHeatCell{},
	}
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
