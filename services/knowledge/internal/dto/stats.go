package dto

import "ego/platform/httpx"

// AdminStatsQuery là kỳ thống kê ?from&to dùng chung cho mọi service.
type AdminStatsQuery = httpx.StatsQuery

// AdminStatsResponse là số liệu học liệu cho màn Admin › Statistics.
type AdminStatsResponse struct {
	Inventory  InventoryStats `json:"inventory"`
	Flashcards FlashcardStats `json:"flashcards"`
	Grammar    GrammarStats   `json:"grammar"`
}

// InventoryStats là số lượng nội dung đang có (dữ liệu seed, không theo kỳ).
type InventoryStats struct {
	Topics       int64 `json:"topics"`
	Sections     int64 `json:"sections"`
	Lessons      int64 `json:"lessons"`
	Transcripts  int64 `json:"transcripts"`
	Words        int64 `json:"words"`
	Themes       int64 `json:"themes"`
	GrammarUnits int64 `json:"grammarUnits"`
}

type FlashcardStats struct {
	Total int64             `json:"total"`
	New   httpx.PeriodCount `json:"new"`
	// Learners: số người có ít nhất một flashcard.
	Learners int64              `json:"learners"`
	Daily    []httpx.DailyCount `json:"daily"`
}

// GrammarStats tính trên lượt đánh dấu hoàn thành unit (cập nhật lần cuối trong kỳ).
type GrammarStats struct {
	Completions httpx.PeriodCount `json:"completions"`
	Learners    int64             `json:"learners"`
	TopUnits    []*UnitStat       `json:"topUnits"`
}

type UnitStat struct {
	UnitID      uint   `json:"unitId"`
	Unit        int    `json:"unit"`
	Title       string `json:"title"`
	Completions int64  `json:"completions"`
}
