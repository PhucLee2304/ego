package dto

import (
	"ego/platform/httpx"
	"ego/services/exams/internal/model"
	"errors"
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

type GetExamQuestionsQuery struct {
	Section model.SectionCode `schema:"section"`
	Part    model.PartCode    `schema:"part"`
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

type GetExamQuestionsResponse struct {
	ID          uint                    `json:"id"`
	Title       string                  `json:"title"`
	Description *string                 `json:"description"`
	Type        string                  `json:"type"`
	Year        *int                    `json:"year"`
	Sections    []*ExamQuestionsSection `json:"sections"`
}

type ExamQuestionsSection struct {
	ID        uint                 `json:"id"`
	Code      string               `json:"code"`
	Title     string               `json:"title"`
	Order     int                  `json:"order"`
	Duration  *int                 `json:"duration"`
	Groups    []*ExamQuestionGroup `json:"groups"`
	Questions []*ExamQuestion      `json:"questions"`
}

type ExamQuestionGroup struct {
	ID          uint            `json:"id"`
	Title       string          `json:"title"`
	Instruction string          `json:"instruction"`
	AudioURL    *string         `json:"audioUrl"`
	ImageURL    *string         `json:"imageUrl"`
	Transcript  string          `json:"transcript"`
	Explanation string          `json:"explanation"`
	Order       int             `json:"order"`
	Questions   []*ExamQuestion `json:"questions"`
}

type ExamQuestion struct {
	ID          uint                  `json:"id"`
	Content     string                `json:"content"`
	Part        *string               `json:"part"`
	Explanation string                `json:"explanation"`
	Order       int                   `json:"order"`
	Options     []*ExamQuestionOption `json:"options"`
}

type ExamQuestionOption struct {
	ID      uint    `json:"id"`
	Key     string  `json:"key"`
	Content *string `json:"content"`
	Order   int     `json:"order"`
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

func ToGetExamQuestionsResponse(exam *model.Exam, query GetExamQuestionsQuery) *GetExamQuestionsResponse {
	sections := make([]*ExamQuestionsSection, 0, len(exam.Sections))

	for _, section := range exam.Sections {
		if query.Section != "" && section.Code != query.Section {
			continue
		}

		sectionResp := &ExamQuestionsSection{
			ID:        section.ID,
			Code:      string(section.Code),
			Title:     section.Title,
			Order:     section.Order,
			Duration:  section.Duration,
			Groups:    []*ExamQuestionGroup{},
			Questions: []*ExamQuestion{},
		}

		for _, group := range section.Groups {
			groupResp := &ExamQuestionGroup{
				ID:          group.ID,
				Title:       group.Title,
				Instruction: group.Instruction,
				AudioURL:    group.AudioURL,
				ImageURL:    group.ImageURL,
				Transcript:  group.Transcript,
				Explanation: group.Explanation,
				Order:       group.Order,
				Questions:   []*ExamQuestion{},
			}

			for _, question := range group.Questions {
				if !matchQuestionPart(question, query.Part) {
					continue
				}
				groupResp.Questions = append(groupResp.Questions, toExamQuestion(question))
			}

			if len(groupResp.Questions) > 0 {
				sort.Slice(groupResp.Questions, func(i, j int) bool {
					return groupResp.Questions[i].Order < groupResp.Questions[j].Order
				})
				sectionResp.Groups = append(sectionResp.Groups, groupResp)
			}
		}

		for _, question := range section.Questions {
			if question.GroupID != nil {
				continue
			}
			if !matchQuestionPart(question, query.Part) {
				continue
			}
			sectionResp.Questions = append(sectionResp.Questions, toExamQuestion(question))
		}

		sort.Slice(sectionResp.Groups, func(i, j int) bool {
			return sectionResp.Groups[i].Order < sectionResp.Groups[j].Order
		})
		sort.Slice(sectionResp.Questions, func(i, j int) bool {
			return sectionResp.Questions[i].Order < sectionResp.Questions[j].Order
		})

		if len(sectionResp.Groups) > 0 || len(sectionResp.Questions) > 0 {
			sections = append(sections, sectionResp)
		}
	}

	sort.Slice(sections, func(i, j int) bool {
		return sections[i].Order < sections[j].Order
	})

	return &GetExamQuestionsResponse{
		ID:          exam.ID,
		Title:       exam.Title,
		Description: exam.Description,
		Type:        string(exam.Type),
		Year:        exam.Year,
		Sections:    sections,
	}
}

func (q *GetExamsQuery) Normalize() {
	q.PaginationQuery.Normalize()

	q.Type = model.ExamType(strings.ToUpper(strings.TrimSpace(string(q.Type))))
	if q.Type == "" {
		q.Type = model.ExamTypeTHPT
	}
}

func (q *GetExamQuestionsQuery) Normalize() {
	q.Section = model.SectionCode(strings.ToUpper(strings.TrimSpace(string(q.Section))))
	q.Part = model.PartCode(strings.TrimSpace(string(q.Part)))
}

func (q GetExamQuestionsQuery) Validate() error {
	if q.Section != "" {
		switch q.Section {
		case model.SectionCodeFull, model.SectionCodeListening, model.SectionCodeReading:
		default:
			return errors.New("[ERROR] Invalid section")
		}
	}

	if q.Part != "" {
		switch q.Part {
		case model.Part1, model.Part2, model.Part3, model.Part4, model.Part5, model.Part6, model.Part7:
		default:
			return errors.New("[ERROR] Invalid part")
		}
	}

	if q.Section != "" && q.Part != "" {
		return errors.New("[ERROR] section and part cannot be used together")
	}

	return nil
}

func ValidateGetExamQuestionsQueryForExam(examType model.ExamType, query GetExamQuestionsQuery) error {
	if query.Part != "" && examType != model.ExamTypeTOEIC {
		return errors.New("[ERROR] part filter is only supported for TOEIC exams")
	}

	if examType == model.ExamTypeTHPT && query.Section != "" && query.Section != model.SectionCodeFull {
		return errors.New("[ERROR] THPT exams only support FULL section")
	}

	if examType == model.ExamTypeTOEIC && query.Section == model.SectionCodeFull {
		return errors.New("[ERROR] TOEIC exams do not support FULL section")
	}

	return nil
}

func matchQuestionPart(question model.Question, part model.PartCode) bool {
	if part == "" {
		return true
	}
	if question.Part == nil {
		return false
	}
	return *question.Part == part
}

func toExamQuestion(question model.Question) *ExamQuestion {
	var part *string
	if question.Part != nil {
		value := string(*question.Part)
		part = &value
	}

	options := make([]*ExamQuestionOption, 0, len(question.Options))
	for _, option := range question.Options {
		options = append(options, &ExamQuestionOption{
			ID:      option.ID,
			Key:     string(option.Key),
			Content: option.Content,
			Order:   option.Order,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Order < options[j].Order
	})

	return &ExamQuestion{
		ID:          question.ID,
		Content:     question.Content,
		Part:        part,
		Explanation: question.Explanation,
		Order:       question.Order,
		Options:     options,
	}
}
