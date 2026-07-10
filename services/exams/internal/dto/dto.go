package dto

import (
	"ego/platform/httpx"
	"ego/services/exams/internal/model"
	"sort"
	"strings"
	"time"
)

type GetExamsResponse struct {
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

type ExamSectionSummary struct {
	ID            uint   `json:"id"`
	Code          string `json:"code"`
	Title         string `json:"title"`
	Order         int    `json:"order"`
	Duration      *int   `json:"duration"`
	QuestionCount int    `json:"questionCount"`
}

type ExamPartSummary struct {
	Part          string `json:"part"`
	SectionCode   string `json:"sectionCode"`
	QuestionCount int    `json:"questionCount"`
}

type GetExamResponse struct {
	ID             uint                  `json:"id"`
	Title          string                `json:"title"`
	Description    *string               `json:"description"`
	IsPublic       bool                  `json:"isPublic"`
	Type           string                `json:"type"`
	Year           *int                  `json:"year"`
	CreatedAt      time.Time             `json:"createdAt"`
	UpdatedAt      time.Time             `json:"updatedAt"`
	Duration       int                   `json:"duration"`
	TotalSections  int                   `json:"totalSections"`
	TotalQuestions int                   `json:"totalQuestions"`
	Sections       []*ExamSectionSummary `json:"sections"`
	Parts          []*ExamPartSummary    `json:"parts"`
}

func ToGetExamsResponse(exam *model.Exam) *GetExamsResponse {
	return &GetExamsResponse{
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

func ToGetExamResponse(exam *model.Exam) *GetExamResponse {
	sections := make([]*ExamSectionSummary, 0, len(exam.Sections))
	partsMap := make(map[string]*ExamPartSummary)
	totalDuration := 0
	totalQuestions := 0

	for _, section := range exam.Sections {
		questionCount := len(section.Questions)
		totalQuestions += questionCount
		if section.Duration != nil {
			totalDuration += *section.Duration
		}

		sections = append(sections, &ExamSectionSummary{
			ID:            section.ID,
			Code:          string(section.Code),
			Title:         section.Title,
			Order:         section.Order,
			Duration:      section.Duration,
			QuestionCount: questionCount,
		})

		for _, question := range section.Questions {
			if question.Part == nil || strings.TrimSpace(string(*question.Part)) == "" {
				continue
			}

			partKey := string(*question.Part)
			partSummary, ok := partsMap[partKey]
			if !ok {
				partSummary = &ExamPartSummary{
					Part:        "PART " + partKey,
					SectionCode: string(section.Code),
				}
				partsMap[partKey] = partSummary
			}
			partSummary.QuestionCount++
		}
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	parts := make([]*ExamPartSummary, 0, len(partsMap))
	for _, part := range partsMap {
		parts = append(parts, part)
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].Part < parts[j].Part
	})

	return &GetExamResponse{
		ID:             exam.ID,
		Title:          exam.Title,
		Description:    exam.Description,
		IsPublic:       exam.IsPublic,
		Type:           string(exam.Type),
		Year:           exam.Year,
		Duration:       totalDuration,
		TotalSections:  len(sections),
		TotalQuestions: totalQuestions,
		Sections:       sections,
		Parts:          parts,
		CreatedAt:      exam.CreatedAt,
		UpdatedAt:      exam.UpdatedAt,
	}
}

func (q *GetExamsQuery) Normalize() {
	q.PaginationQuery.Normalize()

	q.Type = model.ExamType(strings.ToUpper(strings.TrimSpace(string(q.Type))))
	if q.Type == "" {
		q.Type = model.ExamTypeTHPT
	}
}
