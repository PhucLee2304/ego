package dto

import "ego/platform/httpx"

// AdminStatsQuery là kỳ thống kê ?from&to, dùng chung kiểu httpx.StatsQuery với mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là số liệu người dùng cho màn Admin › Users. Mọi số đều bỏ qua tài khoản đã xoá mềm.
type AdminStatsResponse struct {
	// Total, Admins, Clients, Blocked là trạng thái hiện tại, không phụ thuộc kỳ.
	// Clients là mọi tài khoản không phải ADMIN.
	Total   int64 `json:"total"`
	Admins  int64 `json:"admins"`
	Clients int64 `json:"clients"`
	Blocked int64 `json:"blocked"`
	// Signups: số tài khoản tạo trong kỳ (Current) và trong kỳ liền trước cùng độ dài (Previous).
	Signups httpx.PeriodCount `json:"signups"`
	// BlockedInPeriod: số tài khoản đang bị khoá có thời điểm khoá nằm trong kỳ.
	// Tài khoản đã được mở khoá không được tính, vì blocked_at của nó đã về NULL.
	BlockedInPeriod int64 `json:"blockedInPeriod"`
	// UsersBefore: số tài khoản tạo trước đầu kỳ, là mốc để cộng dồn đường tăng trưởng tích luỹ.
	UsersBefore int64 `json:"usersBefore"`
	// Daily: số đăng ký theo từng ngày trong kỳ (giờ Việt Nam), sắp theo ngày tăng dần.
	// Ngày không có đăng ký không có phần tử, client tự điền 0. Luôn là [] chứ không null.
	Daily []httpx.DailyCount `json:"daily"`
	// Heatmap: số đăng ký trong kỳ theo từng ô (thứ, giờ). Ô không có đăng ký không có phần tử.
	// Luôn là [] chứ không null.
	Heatmap []*SignupHeatCell `json:"heatmap"`
}

// SignupHeatCell là số đăng ký trong kỳ tại một ô của heatmap, tính theo giờ Việt Nam.
type SignupHeatCell struct {
	Weekday int   `json:"weekday"` // Thứ theo chuẩn ISO: 1 = thứ Hai … 7 = Chủ nhật.
	Hour    int   `json:"hour"`    // Giờ trong ngày, 0–23.
	Count   int64 `json:"count"`
}
