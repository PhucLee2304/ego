package dto

import (
	"sort"

	"ego/platform/valuex"
	"ego/services/learning/internal/model"
)

type GetExamQuestionsQuery struct {
	Section model.SectionCode `schema:"section" validate:"omitempty,oneof=FULL LISTENING READING"`
	Part    model.PartCode    `schema:"part" validate:"omitempty,oneof=1 2 3 4 5 6 7"`
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
	ImageURLs   []string        `json:"imageUrls"`
	Transcript  string          `json:"transcript"`
	Explanation string          `json:"explanation"`
	Order       int             `json:"order"`
	Questions   []*ExamQuestion `json:"questions"`
}

type ExamQuestion struct {
	ID                uint                  `json:"id"`
	Content           string                `json:"content"`
	Part              *string               `json:"part"`
	Number            *int                  `json:"number,omitempty"` // số câu gốc (TOEIC 1–200)
	Explanation       string                `json:"explanation"`
	Order             int                   `json:"order"`
	ExcludedFromScore *bool                 `json:"excludedFromScore,omitempty"`
	SelectedOptionID  *uint                 `json:"selectedOptionId,omitempty"`
	CorrectOptionID   *uint                 `json:"correctOptionId,omitempty"`
	IsCorrect         *bool                 `json:"isCorrect,omitempty"`
	Options           []*ExamQuestionOption `json:"options"`
}

type ExamQuestionOption struct {
	ID        uint    `json:"id"`
	Key       string  `json:"key"`
	Content   *string `json:"content"`
	Order     int     `json:"order"`
	IsCorrect bool    `json:"-"`
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

		groupParts := make(map[*ExamQuestionGroup]string, len(section.Groups))
		for _, group := range section.Groups {
			groupResp := &ExamQuestionGroup{
				ID:          group.ID,
				Title:       group.Title,
				Instruction: group.Instruction,
				AudioURL:    group.AudioURL,
				ImageURLs:   group.ImageURLs,
				Transcript:  group.Transcript,
				Explanation: group.Explanation,
				Order:       group.Order,
				Questions:   []*ExamQuestion{},
			}

			for _, question := range group.Questions {
				if query.Part != "" && (question.Part == nil || *question.Part != query.Part) {
					continue
				}
				groupResp.Questions = append(groupResp.Questions, toExamQuestion(question))
			}

			if len(groupResp.Questions) > 0 {
				sort.Slice(groupResp.Questions, func(i, j int) bool {
					return groupResp.Questions[i].Order < groupResp.Questions[j].Order
				})
				groupParts[groupResp] = groupPart(group)
				sectionResp.Groups = append(sectionResp.Groups, groupResp)
			}
		}

		for _, question := range section.Questions {
			if question.GroupID != nil {
				continue
			}
			if query.Part != "" && (question.Part == nil || *question.Part != query.Part) {
				continue
			}
			sectionResp.Questions = append(sectionResp.Questions, toExamQuestion(question))
		}

		sort.SliceStable(sectionResp.Groups, func(i, j int) bool {
			left, right := sectionResp.Groups[i], sectionResp.Groups[j]
			if groupParts[left] != groupParts[right] {
				return groupParts[left] < groupParts[right]
			}
			return left.Order < right.Order
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

// groupPart lấy Part của group; group cũ chưa có cột Part thì lấy theo câu đầu tiên.
// Part là "1".."7" nên so sánh chuỗi đúng thứ tự; group không có part ("") xếp trước.
func groupPart(group model.Group) string {
	if group.Part != nil {
		return string(*group.Part)
	}
	for _, question := range group.Questions {
		if question.Part != nil {
			return string(*question.Part)
		}
	}
	return ""
}

func toExamQuestion(question model.Question) *ExamQuestion {
	excludedFromScore := question.ExcludedFromScore
	options := make([]*ExamQuestionOption, 0, len(question.Options))
	for _, option := range question.Options {
		options = append(options, &ExamQuestionOption{
			ID:        option.ID,
			Key:       string(option.Key),
			Content:   option.Content,
			Order:     option.Order,
			IsCorrect: option.IsCorrect,
		})
	}
	sort.Slice(options, func(i, j int) bool {
		return options[i].Order < options[j].Order
	})

	return &ExamQuestion{
		ID:                question.ID,
		Content:           question.Content,
		ExcludedFromScore: &excludedFromScore,
		Part: valuex.MapPtr(question.Part, func(value model.PartCode) string {
			return string(value)
		}),
		Number:      question.Number,
		Explanation: question.Explanation,
		Order:       question.Order,
		Options:     options,
	}
}

func filterSectionsByParts(sections []*ExamQuestionsSection, parts []model.PartCode) []*ExamQuestionsSection {
	if len(parts) == 0 {
		return sections
	}

	allowedParts := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		allowedParts[string(part)] = struct{}{}
	}

	filteredSections := make([]*ExamQuestionsSection, 0, len(sections))
	for _, section := range sections {
		filteredGroups := make([]*ExamQuestionGroup, 0, len(section.Groups))
		for _, group := range section.Groups {
			filteredQuestions := make([]*ExamQuestion, 0, len(group.Questions))
			for _, question := range group.Questions {
				if question.Part == nil {
					continue
				}
				if _, ok := allowedParts[*question.Part]; ok {
					filteredQuestions = append(filteredQuestions, question)
				}
			}
			if len(filteredQuestions) > 0 {
				group.Questions = filteredQuestions
				filteredGroups = append(filteredGroups, group)
			}
		}

		filteredQuestions := make([]*ExamQuestion, 0, len(section.Questions))
		for _, question := range section.Questions {
			if question.Part == nil {
				continue
			}
			if _, ok := allowedParts[*question.Part]; ok {
				filteredQuestions = append(filteredQuestions, question)
			}
		}

		section.Groups = filteredGroups
		section.Questions = filteredQuestions
		if len(section.Groups) > 0 || len(section.Questions) > 0 {
			filteredSections = append(filteredSections, section)
		}
	}

	return filteredSections
}
