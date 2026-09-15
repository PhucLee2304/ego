package dto

import (
	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/exams/internal/model"
	"encoding/json"
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

type GetAttemptsQuery struct {
	httpx.PaginationQuery
	Status model.AttemptStatus `schema:"status"`
}

type CreateExamAttemptRequest struct {
	Mode     model.AttemptMode      `json:"mode"`
	Section  *model.SectionCode     `json:"section,omitempty"`
	Parts    []model.PartCode       `json:"parts,omitempty"`
	Duration *model.AttemptDuration `json:"duration,omitempty"`
}

type SubmitAttemptRequest struct {
	Answers []SubmitAttemptAnswer `json:"answers"`
}

type SubmitAttemptAnswer struct {
	QuestionID       uint  `json:"questionId"`
	SelectedOptionID *uint `json:"selectedOptionId"`
}

type AttemptResponse struct {
	ID             uint                    `json:"id"`
	ExamID         uint                    `json:"examId"`
	Exam           *AttemptExamSummary     `json:"exam,omitempty"`
	Mode           string                  `json:"mode"`
	Status         string                  `json:"status"`
	Section        *string                 `json:"section,omitempty"`
	Parts          []string                `json:"parts,omitempty"`
	Duration       *int                    `json:"duration,omitempty"`
	StartedAt      time.Time               `json:"startedAt"`
	ExpiresAt      *time.Time              `json:"expiresAt,omitempty"`
	SubmittedAt    *time.Time              `json:"submittedAt,omitempty"`
	TotalQuestions int                     `json:"totalQuestions"`
	AnsweredCount  int                     `json:"answeredCount"`
	CorrectAnswers *int                    `json:"correctAnswers,omitempty"`
	Score          *float64                `json:"score,omitempty"`
	CreatedAt      time.Time               `json:"createdAt"`
	UpdatedAt      time.Time               `json:"updatedAt"`
	Sections       []*ExamQuestionsSection `json:"sections,omitempty"`
}

type AttemptExamSummary struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Type        string  `json:"type"`
	Year        *int    `json:"year"`
}

type UpdateAttemptAnswerResponse struct {
	AttemptID        uint      `json:"attemptId"`
	QuestionID       uint      `json:"questionId"`
	SelectedOptionID *uint     `json:"selectedOptionId,omitempty"`
	AnsweredAt       time.Time `json:"answeredAt"`
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
	ID               uint                  `json:"id"`
	Content          string                `json:"content"`
	Part             *string               `json:"part"`
	Explanation      string                `json:"explanation"`
	Order            int                   `json:"order"`
	SelectedOptionID *uint                 `json:"selectedOptionId,omitempty"`
	CorrectOptionID  *uint                 `json:"correctOptionId,omitempty"`
	IsCorrect        *bool                 `json:"isCorrect,omitempty"`
	Options          []*ExamQuestionOption `json:"options"`
}

type ExamQuestionOption struct {
	ID        uint    `json:"id"`
	Key       string  `json:"key"`
	Content   *string `json:"content"`
	Order     int     `json:"order"`
	IsCorrect bool    `json:"-"`
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
				if query.Part != "" && (question.Part == nil || *question.Part != query.Part) {
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
			if query.Part != "" && (question.Part == nil || *question.Part != query.Part) {
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

func (q *GetAttemptsQuery) Normalize() {
	q.PaginationQuery.Normalize()
	q.Status = model.AttemptStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
}

func (r *CreateExamAttemptRequest) Normalize() {
	r.Mode = model.AttemptMode(strings.ToUpper(strings.TrimSpace(string(r.Mode))))

	if r.Section != nil {
		value := model.SectionCode(strings.ToUpper(strings.TrimSpace(string(*r.Section))))
		r.Section = &value
	}

	if len(r.Parts) > 0 {
		parts := make([]model.PartCode, 0, len(r.Parts))
		for _, part := range r.Parts {
			value := model.PartCode(strings.TrimSpace(string(part)))
			parts = append(parts, value)
		}
		r.Parts = parts
	}
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

func (q GetAttemptsQuery) Validate() error {
	switch q.Status {
	case model.AttemptStatusInProgress, model.AttemptStatusSubmitted, model.AttemptStatusCancelled:
		return nil
	default:
		return errors.New("[ERROR] Invalid status")
	}
}

func (r CreateExamAttemptRequest) Validate() error {
	switch r.Mode {
	case model.AttemptModePractice, model.AttemptModeTest:
	default:
		return errors.New("[ERROR] Invalid mode")
	}

	if r.Section != nil {
		switch *r.Section {
		case model.SectionCodeFull, model.SectionCodeListening, model.SectionCodeReading:
		default:
			return errors.New("[ERROR] Invalid section")
		}
	}

	seenParts := make(map[model.PartCode]struct{}, len(r.Parts))
	for _, part := range r.Parts {
		switch part {
		case model.Part1, model.Part2, model.Part3, model.Part4, model.Part5, model.Part6, model.Part7:
		default:
			return errors.New("[ERROR] Invalid part")
		}
		if _, ok := seenParts[part]; ok {
			return errors.New("[ERROR] Duplicate part")
		}
		seenParts[part] = struct{}{}
	}

	if r.Section != nil && len(r.Parts) > 0 {
		return errors.New("[ERROR] section and parts cannot be used together")
	}

	return nil
}

func (r SubmitAttemptRequest) Validate() error {
	seenQuestions := make(map[uint]struct{}, len(r.Answers))
	for _, answer := range r.Answers {
		if answer.QuestionID == 0 {
			return errors.New("[ERROR] Invalid question ID")
		}
		if answer.SelectedOptionID != nil && *answer.SelectedOptionID == 0 {
			return errors.New("[ERROR] Invalid option ID")
		}
		if _, ok := seenQuestions[answer.QuestionID]; ok {
			return errors.New("[ERROR] Duplicate question")
		}
		seenQuestions[answer.QuestionID] = struct{}{}
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

func ValidateCreateExamAttemptRequestForExam(examType model.ExamType, req CreateExamAttemptRequest) error {
	if req.Mode == model.AttemptModeTest {
		if req.Section != nil {
			return errors.New("[ERROR] TEST mode does not support section")
		}
		if len(req.Parts) > 0 {
			return errors.New("[ERROR] TEST mode does not support parts")
		}
		if req.Duration != nil {
			return errors.New("[ERROR] TEST mode does not accept duration")
		}
		return nil
	}

	if len(req.Parts) > 0 && examType != model.ExamTypeTOEIC {
		return errors.New("[ERROR] parts are only supported for TOEIC exams")
	}

	if examType == model.ExamTypeTHPT && req.Section != nil && *req.Section != model.SectionCodeFull {
		return errors.New("[ERROR] THPT exams only support FULL section")
	}

	if examType == model.ExamTypeTOEIC && req.Section != nil && *req.Section == model.SectionCodeFull {
		return errors.New("[ERROR] TOEIC exams do not support FULL section")
	}

	if !model.IsAllowedPracticeDuration(valuex.MapPtr(req.Duration, func(value model.AttemptDuration) int {
		return int(value)
	})) {
		return errors.New("[ERROR] Invalid duration")
	}

	return nil
}

func toExamQuestion(question model.Question) *ExamQuestion {
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
		ID:      question.ID,
		Content: question.Content,
		Part: valuex.MapPtr(question.Part, func(value model.PartCode) string {
			return string(value)
		}),
		Explanation: question.Explanation,
		Order:       question.Order,
		Options:     options,
	}
}

func ToAttemptResponse(attempt *model.Attempt, now time.Time) *AttemptResponse {
	parts := make([]string, 0, len(attempt.PartCodes))
	for _, part := range attempt.PartCodes {
		parts = append(parts, string(part))
	}

	var exam *AttemptExamSummary
	if attempt.Exam.ID != 0 {
		exam = &AttemptExamSummary{
			ID:          attempt.Exam.ID,
			Title:       attempt.Exam.Title,
			Description: attempt.Exam.Description,
			Type:        string(attempt.Exam.Type),
			Year:        attempt.Exam.Year,
		}
	}

	return &AttemptResponse{
		ID:     attempt.ID,
		ExamID: attempt.ExamID,
		Exam:   exam,
		Mode:   string(attempt.Mode),
		Status: string(attempt.Status),
		Section: valuex.MapPtr(attempt.SectionCode, func(value model.SectionCode) string {
			return string(value)
		}),
		Parts:          parts,
		Duration:       attempt.Duration,
		StartedAt:      attempt.StartedAt,
		ExpiresAt:      attempt.ExpiresAt,
		SubmittedAt:    attempt.SubmittedAt,
		TotalQuestions: attempt.TotalQuestions,
		AnsweredCount:  len(attempt.Answers),
		CorrectAnswers: attempt.CorrectAnswers,
		Score:          attempt.Score,
		CreatedAt:      attempt.CreatedAt,
		UpdatedAt:      attempt.UpdatedAt,
		Sections:       nil,
	}
}

func ToAttemptResponseWithQuestions(attempt *model.Attempt) *AttemptResponse {
	return toAttemptResponseWithQuestions(attempt, false)
}

func ToAttemptHistoryResponse(attempt *model.Attempt) *AttemptResponse {
	return toAttemptResponseWithQuestions(attempt, attempt.Status == model.AttemptStatusSubmitted)
}

func MarshalAttemptHistory(attempt *model.Attempt) ([]byte, error) {
	return json.Marshal(ToAttemptHistoryResponse(attempt))
}

func UnmarshalAttemptHistory(data []byte) (*AttemptResponse, error) {
	var response AttemptResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func toAttemptResponseWithQuestions(attempt *model.Attempt, includeResults bool) *AttemptResponse {
	query := GetExamQuestionsQuery{}
	if attempt.SectionCode != nil {
		query.Section = *attempt.SectionCode
	}

	questionsResp := ToGetExamQuestionsResponse(&attempt.Exam, query)
	questionsResp.Sections = filterSectionsByParts(questionsResp.Sections, attempt.PartCodes)
	hideSolutions := attempt.Status == model.AttemptStatusInProgress
	answers := make(map[uint]model.AttemptAnswer, len(attempt.Answers))
	for _, answer := range attempt.Answers {
		answers[answer.QuestionID] = answer
	}

	for _, section := range questionsResp.Sections {
		for _, group := range section.Groups {
			if hideSolutions {
				group.Transcript = ""
				group.Explanation = ""
			}
			for _, question := range group.Questions {
				if hideSolutions {
					question.Explanation = ""
				}
				applyAttemptAnswer(question, answers[question.ID], includeResults)
			}
		}
		for _, question := range section.Questions {
			if hideSolutions {
				question.Explanation = ""
			}
			applyAttemptAnswer(question, answers[question.ID], includeResults)
		}
	}

	resp := ToAttemptResponse(attempt, time.Now())
	resp.Sections = questionsResp.Sections

	return resp
}

func applyAttemptAnswer(question *ExamQuestion, answer model.AttemptAnswer, includeResults bool) {
	question.SelectedOptionID = answer.SelectedOptionID
	if !includeResults {
		return
	}

	question.IsCorrect = answer.IsCorrect
	for _, option := range question.Options {
		if option.IsCorrect {
			value := option.ID
			question.CorrectOptionID = &value
			break
		}
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
