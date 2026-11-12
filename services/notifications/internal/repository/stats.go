package repository

import (
	"context"
	"strings"

	"ego/platform/httpx"
	"ego/services/notifications/internal/dto"
)

// statsTotalsSQL đếm tổng thông báo đã gửi và đã đọc trong kỳ [from, to) và kỳ liền trước
// [prev_from, prev_to), bằng một lần quét bảng (FILTER thay cho 4 câu COUNT riêng). Hai số của kỳ trước
// để dashboard admin so sánh với kỳ trước. Bỏ qua thông báo đã xoá mềm.
const statsTotalsSQL = `
SELECT
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to) AS sent,
	COUNT(*) FILTER (WHERE created_at >= @from AND created_at < @to AND read_at IS NOT NULL) AS read,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to) AS prev_sent,
	COUNT(*) FILTER (WHERE created_at >= @prev_from AND created_at < @prev_to AND read_at IS NOT NULL) AS prev_read
FROM notifications WHERE deleted_at IS NULL`

// statsDailySQL đếm số đã gửi và đã đọc theo từng ngày trong kỳ [from, to), cho biểu đồ theo ngày.
// Ngày được cắt theo múi giờ @tz (httpx.StatsTimeZone) thay vì UTC, để thông báo lúc 0h–7h sáng giờ VN
// không bị tính sang hôm trước. COUNT(read_at) chỉ đếm dòng có read_at khác NULL, tức số đã đọc. Ngày
// không có thông báo nào thì không có dòng trong kết quả.
const statsDailySQL = `
SELECT to_char(created_at AT TIME ZONE @tz, 'YYYY-MM-DD') AS date, COUNT(*) AS sent, COUNT(read_at) AS read
FROM notifications WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY date ORDER BY date`

// statsTypesSQL đếm số đã gửi và đã đọc theo data.type (POST_LIKED, ASSIGNMENT, ROOM…) trong kỳ
// [from, to), để service gộp thành các nhóm trong model.CategoryTypes. Thông báo không có type được
// gom vào type rỗng thay vì NULL để Scan vào string không lỗi.
const statsTypesSQL = `
SELECT COALESCE(data->>'type', '') AS type, COUNT(*) AS sent, COUNT(read_at) AS read
FROM notifications WHERE deleted_at IS NULL AND created_at >= @from AND created_at < @to
GROUP BY type`

// statsDevicesSQL đếm số thiết bị đang đăng ký nhận push và số user có ít nhất một thiết bị. Không lọc
// theo kỳ: đây là trạng thái hiện tại của bảng device_tokens.
const statsDevicesSQL = `SELECT COUNT(*) AS devices, COUNT(DISTINCT user_id) AS device_users FROM device_tokens`

// StatsTypeCount là số thông báo trong kỳ theo data.type; service gom thành nhóm.
type StatsTypeCount struct {
	Type       string
	Sent, Read int64
}

// **Reviewed**
func (r *repository) GetAdminStats(ctx context.Context, rng httpx.StatsRange) (*dto.AdminStatsResponse, []StatsTypeCount, error) {
	db := r.db.WithContext(ctx)
	args := map[string]any{
		"from":      rng.From,
		"to":        rng.To,
		"prev_from": rng.PrevFrom,
		"prev_to":   rng.PrevTo,
		"tz":        httpx.StatsTimeZone,
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
