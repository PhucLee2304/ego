package dto

import (
	"time"

	"ego/platform/httpx"
)

// AdminStatsQuery là kỳ thống kê ?from&to dùng chung cho mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là toàn bộ số liệu learning cho màn Admin › Statistics.
type AdminStatsResponse struct {
	Classrooms  ClassroomStats       `json:"classrooms"`
	Assignments AssignmentStats      `json:"assignments"`
	Attempts    AttemptStats         `json:"attempts"`
	Integrity   IntegrityStats       `json:"integrity"`
	Rooms       RoomStats            `json:"rooms"`
	List        []*ClassroomStatsRow `json:"list"`
}

type ClassroomStats struct {
	// Active: lớp đang hoạt động tạo trước cuối kỳ; Previous là trước cuối kỳ trước.
	Active   httpx.PeriodCount `json:"active"`
	Archived int64             `json:"archived"`
	// Teachers: số người phụ trách ít nhất một lớp đang hoạt động.
	Teachers int64 `json:"teachers"`
	// Students: số người là thành viên đã duyệt của ít nhất một lớp, tính tới cuối kỳ.
	Students        httpx.PeriodCount `json:"students"`
	PendingRequests int64             `json:"pendingRequests"`
}

type AssignmentStats struct {
	Total int64 `json:"total"`
	Open  int64 `json:"open"`
	// Submissions: bài đã chấm nộp trong kỳ; OnTime + Late = Submissions.Current.
	Submissions httpx.PeriodCount `json:"submissions"`
	OnTime      int64             `json:"onTime"`
	Late        int64             `json:"late"`
	// NotSubmitted: thành viên chưa nộp của các bài đang mở.
	NotSubmitted int64       `json:"notSubmitted"`
	AverageScore PeriodScore `json:"averageScore"`
	// ScoreDistribution có 10 ô điểm [0,1) … [9,10] của bài nộp trong kỳ.
	ScoreDistribution []int64 `json:"scoreDistribution"`
}

type PeriodScore struct {
	Current  *float64 `json:"current"`
	Previous *float64 `json:"previous"`
}

type AttemptStats struct {
	// Total: lượt đã nộp trong kỳ (mọi mode).
	Total  httpx.PeriodCount    `json:"total"`
	ByMode []httpx.LabeledCount `json:"byMode"`
	Daily  []*DailyAttempts     `json:"daily"`
	// THPT (thang 10) và TOEIC (thang 990): top 10 đề nhiều lượt tự do nhất trong kỳ.
	THPT  []*ExamAverage `json:"thpt"`
	TOEIC []*ExamAverage `json:"toeic"`
}

type DailyAttempts struct {
	Date       string `json:"date"`
	Assignment int64  `json:"assignment"`
	Practice   int64  `json:"practice"`
	Test       int64  `json:"test"`
}

type ExamAverage struct {
	ExamID       uint    `json:"examId"`
	Title        string  `json:"title"`
	Attempts     int64   `json:"attempts"`
	AverageScore float64 `json:"averageScore"`
}

type IntegrityStats struct {
	// ByType: vi phạm (level VIOLATION) ghi nhận trong kỳ theo loại sự kiện.
	ByType     []httpx.LabeledCount `json:"byType"`
	Terminated int64                `json:"terminated"`
}

type RoomStats struct {
	// Sessions: buổi học trực tuyến đã kết thúc, bắt đầu trong kỳ.
	Sessions        int64   `json:"sessions"`
	Hours           float64 `json:"hours"`
	AvgParticipants float64 `json:"avgParticipants"`
}

// ClassroomStatsRow là một dòng bảng "Lớp học"; số liệu tính trên toàn thời gian.
type ClassroomStatsRow struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Code         string    `json:"code"`
	Teacher      *UserInfo `json:"teacher"`
	Students     int64     `json:"students"`
	Assignments  int64     `json:"assignments"`
	Submissions  int64     `json:"submissions"`
	AverageScore *float64  `json:"averageScore"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"createdAt"`
}
