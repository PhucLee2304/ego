package dto

import "ego/platform/httpx"

// AdminStatsQuery là kỳ thống kê ?from&to dùng chung cho mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là số liệu người dùng cho màn Admin › Users.
type AdminStatsResponse struct {
	// Total, Admins, Clients, Blocked là số hiện tại, không phụ thuộc kỳ.
	Total   int64 `json:"total"`
	Admins  int64 `json:"admins"`
	Clients int64 `json:"clients"`
	Blocked int64 `json:"blocked"`
	// Signups: tài khoản tạo trong kỳ và kỳ trước cùng độ dài.
	Signups httpx.PeriodCount `json:"signups"`
	// BlockedInPeriod: tài khoản đang khoá có thời điểm khoá trong kỳ.
	BlockedInPeriod int64 `json:"blockedInPeriod"`
	// UsersBefore: số tài khoản tạo trước đầu kỳ, là mốc của đường tăng trưởng tích luỹ.
	UsersBefore int64              `json:"usersBefore"`
	Daily       []httpx.DailyCount `json:"daily"`
	Heatmap     []*SignupHeatCell  `json:"heatmap"`
}

// SignupHeatCell là số đăng ký trong kỳ theo thứ (1 = thứ Hai … 7 = Chủ nhật) và giờ (0–23), giờ Việt Nam.
type SignupHeatCell struct {
	Weekday int   `json:"weekday"`
	Hour    int   `json:"hour"`
	Count   int64 `json:"count"`
}
