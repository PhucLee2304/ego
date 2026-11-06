package httpx

import (
	"time"
)

// StatsLocation là múi giờ dùng để cắt ngày cho mọi API thống kê admin.
var StatsLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("ICT", 7*60*60)
	}
	return loc
}()

// StatsTimeZone là tên múi giờ cho SQL: to_char(col AT TIME ZONE StatsTimeZone, 'YYYY-MM-DD').
const StatsTimeZone = "Asia/Ho_Chi_Minh"

// StatsQuery là kỳ thống kê ?from=YYYY-MM-DD&to=YYYY-MM-DD (tính cả hai ngày, giờ Việt Nam).
// Bỏ trống cả hai là toàn thời gian.
type StatsQuery struct {
	From string `schema:"from" validate:"omitempty,datetime=2006-01-02"`
	To   string `schema:"to" validate:"required_with=From,omitempty,datetime=2006-01-02"`
}

// StatsRange là khoảng [From, To) và kỳ liền trước cùng độ dài [PrevFrom, PrevTo).
// Toàn thời gian thì kỳ trước rỗng (PrevFrom = PrevTo) nên mọi số của kỳ trước bằng 0.
type StatsRange struct {
	From     time.Time
	To       time.Time
	PrevFrom time.Time
	PrevTo   time.Time
}

// Range đổi query thành khoảng thời gian; query đã qua DecodeQuery nên ngày luôn hợp lệ.
func (q StatsQuery) Range() StatsRange {
	if q.From == "" {
		from := time.Unix(0, 0)
		to := time.Now().AddDate(1, 0, 0)
		return StatsRange{From: from, To: to, PrevFrom: from, PrevTo: from}
	}
	from, _ := time.ParseInLocation(time.DateOnly, q.From, StatsLocation)
	to, _ := time.ParseInLocation(time.DateOnly, q.To, StatsLocation)
	to = to.AddDate(0, 0, 1)
	if to.Before(from) {
		from, to = to.AddDate(0, 0, -1), from.AddDate(0, 0, 1)
	}
	length := to.Sub(from)
	return StatsRange{From: from, To: to, PrevFrom: from.Add(-length), PrevTo: from}
}

// DailyCount là số đếm của một ngày (YYYY-MM-DD, giờ Việt Nam). Ngày không có dữ liệu không có mặt.
type DailyCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// LabeledCount là số đếm theo một nhãn (loại, nhóm…).
type LabeledCount struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// PeriodCount là số của kỳ đang xem kèm số của kỳ trước cùng độ dài.
type PeriodCount struct {
	Current  int64 `json:"current"`
	Previous int64 `json:"previous"`
}
