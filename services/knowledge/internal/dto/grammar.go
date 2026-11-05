package dto

import (
	"time"

	"ego/services/knowledge/internal/model"
)

type Practice struct {
	ID          uint     `json:"id"`
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	Correct     string   `json:"correct"`
	Explanation string   `json:"explanation"`
	SortOrder   int      `json:"sortOrder"`
}

// UnitSummary là một unit trong danh sách. Total luôn là 1 vì mỗi
// unit có một bộ luyện tập.
type UnitSummary struct {
	ID        uint   `json:"id"`
	Unit      int    `json:"unit"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	Total     int    `json:"total"`
}

type Unit struct {
	ID        uint        `json:"id"`
	Unit      int         `json:"unit"`
	Title     string      `json:"title"`
	Theory    string      `json:"theory"`
	Examples  []string    `json:"examples"`
	Practices []*Practice `json:"practices"`
	Completed bool        `json:"completed"`
}

type GetUnitsResponse struct {
	Data  []*UnitSummary `json:"data"`
	Total int            `json:"total"`
}

type UpdateProgressBody struct {
	Completed bool `json:"completed"`
}

type Progress struct {
	ID        uint      `json:"id"`
	UserID    string    `json:"userId"`
	UnitID    uint      `json:"unitId"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToUnitSummary cần unit đã preload Progresses của người dùng hiện tại.
func ToUnitSummary(unit *model.GrammarUnit) *UnitSummary {
	return &UnitSummary{
		ID:        unit.ID,
		Unit:      unit.Unit,
		Title:     unit.Title,
		Completed: isCompleted(unit),
		Total:     1,
	}
}

// ToUnit cần unit đã preload Examples, Practices và Progresses của người dùng
// hiện tại.
func ToUnit(unit *model.GrammarUnit) *Unit {
	examples := make([]string, len(unit.Examples))
	for i := range unit.Examples {
		examples[i] = unit.Examples[i].Content
	}

	practices := make([]*Practice, len(unit.Practices))
	for i := range unit.Practices {
		practices[i] = ToPractice(&unit.Practices[i])
	}

	return &Unit{
		ID:        unit.ID,
		Unit:      unit.Unit,
		Title:     unit.Title,
		Theory:    unit.Theory,
		Examples:  examples,
		Practices: practices,
		Completed: isCompleted(unit),
	}
}

func ToPractice(practice *model.GrammarPractice) *Practice {
	return &Practice{
		ID:          practice.ID,
		Question:    practice.Question,
		Options:     practice.Options,
		Correct:     practice.Correct,
		Explanation: practice.Explanation,
		SortOrder:   practice.SortOrder,
	}
}

func ToProgress(progress *model.UserUnitProgress) *Progress {
	return &Progress{
		ID:        progress.ID,
		UserID:    progress.UserID,
		UnitID:    progress.UnitID,
		Completed: progress.Completed,
		UpdatedAt: progress.UpdatedAt,
	}
}

// isCompleted: Progresses chỉ chứa dòng của người dùng hiện tại (tối đa 1).
func isCompleted(unit *model.GrammarUnit) bool {
	return len(unit.Progresses) > 0 && unit.Progresses[0].Completed
}
