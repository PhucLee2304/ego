package dto

import "ego/platform/httpx"

// AdminStatsQuery là kỳ thống kê ?from&to dùng chung cho mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là số liệu thông báo cho màn Admin › Statistics.
type AdminStatsResponse struct {
	// Sent: thông báo tạo trong kỳ; Read: trong số đó, đã được đọc.
	Sent       httpx.PeriodCount    `json:"sent"`
	Read       httpx.PeriodCount    `json:"read"`
	Daily      []*DailyNotification `json:"daily"`
	Categories []*CategoryStat      `json:"categories"`
	// Devices: thiết bị đã đăng ký push; DeviceUsers: số người sở hữu.
	Devices     int64 `json:"devices"`
	DeviceUsers int64 `json:"deviceUsers"`
}

type DailyNotification struct {
	Date string `json:"date"`
	Sent int64  `json:"sent"`
	Read int64  `json:"read"`
}

// CategoryStat là thông báo trong kỳ theo nhóm (POSTS, CLASSROOMS, ASSIGNMENTS, ROOMS, EXAMS, OTHER).
type CategoryStat struct {
	Category string `json:"category"`
	Sent     int64  `json:"sent"`
	Read     int64  `json:"read"`
}
