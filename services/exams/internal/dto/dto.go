package dto

import (
	"ego/platform/httpx"
	"ego/services/exams/internal/model"
	"strings"
	"time"
)

type Exam struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	IsPublic    bool      `json:"isPublic"`
	Type        string    `json:"type"`
	Year        *int      `json:"year"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type GetExamsQuery struct {
	httpx.PaginationQuery
	Type model.ExamType `schema:"type"`
}

func ToExamDTO(exam *model.Exam) *Exam {
	return &Exam{
		ID:          exam.ID,
		Title:       exam.Title,
		Description: exam.Description,
		IsPublic:    exam.IsPublic,
		Type:        string(exam.Type),
		Year:        exam.Year,
		CreatedAt:   exam.CreatedAt,
		UpdatedAt:   exam.UpdatedAt,
	}
}

func (q *GetExamsQuery) Normalize() {
	q.PaginationQuery.Normalize()

	q.Type = model.ExamType(strings.ToUpper(strings.TrimSpace(string(q.Type))))
	if q.Type == "" {
		q.Type = model.ExamTypeTHPT
	}
}
