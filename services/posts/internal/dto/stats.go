package dto

import "ego/platform/httpx"

// AdminStatsQuery là kỳ thống kê ?from&to dùng chung cho mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là số liệu bài cộng đồng cho màn Admin › Statistics (không tính bài trong lớp, trừ ClassroomPosts).
type AdminStatsResponse struct {
	// TotalPosts: bài cộng đồng đang hiện (mọi thời gian); NewPosts: bài tạo trong kỳ.
	TotalPosts     int64                `json:"totalPosts"`
	NewPosts       httpx.PeriodCount    `json:"newPosts"`
	HiddenPosts    int64                `json:"hiddenPosts"`
	ClassroomPosts int64                `json:"classroomPosts"`
	Comments       httpx.PeriodCount    `json:"comments"`
	Likes          httpx.PeriodCount    `json:"likes"`
	Daily          []*DailyPostActivity `json:"daily"`
	Attachments    []*AttachmentStat    `json:"attachments"`
	TopAuthors     []*AuthorStat        `json:"topAuthors"`
}

type DailyPostActivity struct {
	Date     string `json:"date"`
	Posts    int64  `json:"posts"`
	Comments int64  `json:"comments"`
}

// AttachmentStat là tệp đính kèm của bài cộng đồng tạo trong kỳ theo loại (IMAGE, VIDEO, FILE).
type AttachmentStat struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
	Bytes int64  `json:"bytes"`
}

type AuthorStat struct {
	UserID string    `json:"-"`
	User   *UserInfo `json:"user" gorm:"-"`
	Posts  int64     `json:"posts"`
	Likes  int64     `json:"likes"`
}
