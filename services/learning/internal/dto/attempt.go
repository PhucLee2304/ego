package dto

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/model"
)

type GetAttemptsQuery struct {
	httpx.PaginationQuery
	Status model.AttemptStatus `schema:"status" validate:"required,oneof=ACTIVE SUBMITTED CANCELLED"`
}

func (q *GetAttemptsQuery) Normalize() {
	q.PaginationQuery.Normalize()
	q.Status = model.AttemptStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
}

type CreateExamAttemptRequest struct {
	Mode        model.AttemptMode        `json:"mode,omitempty" validate:"omitempty,oneof=PRACTICE TEST ASSIGNMENT"`
	Section     *model.SectionCode       `json:"section,omitempty" validate:"omitempty,oneof=FULL LISTENING READING"`
	Parts       []model.PartCode         `json:"parts,omitempty" validate:"omitempty,dive,oneof=1 2 3 4 5 6 7"`
	Duration    *model.AttemptDuration   `json:"duration,omitempty" validate:"omitempty,oneof=10 15 20 30 45 60 75 90 120"`
	ContextType model.AttemptContextType `json:"contextType" validate:"required,oneof=STANDALONE CLASSROOM_ASSIGNMENT"`
	ContextID   *uint                    `json:"contextId,omitempty" validate:"omitempty,gt=0"`
	// Bắt buộc khi assignment có giám sát (PLAN_ATTEMPT_INTEGRITY.md mục 1.7).
	IntegrityConsent *IntegrityConsentRequest `json:"integrityConsent,omitempty"`
}

func (r *CreateExamAttemptRequest) Normalize() {
	r.Mode = model.AttemptMode(strings.ToUpper(strings.TrimSpace(string(r.Mode))))
	r.ContextType = model.AttemptContextType(strings.ToUpper(strings.TrimSpace(string(r.ContextType))))
	if r.ContextType == model.AttemptContextTypeClassroomAssignment && r.Mode == "" {
		r.Mode = model.AttemptModeAssignment
	}

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

func (r CreateExamAttemptRequest) Validate() error {
	switch r.ContextType {
	case model.AttemptContextTypeStandalone:
		if r.Mode != model.AttemptModePractice && r.Mode != model.AttemptModeTest {
			return errors.New("[ERROR] STANDALONE attempts must use PRACTICE or TEST mode")
		}
		if r.ContextID != nil {
			return errors.New("[ERROR] STANDALONE context does not accept contextId")
		}
	case model.AttemptContextTypeClassroomAssignment:
		if r.ContextID == nil || *r.ContextID == 0 {
			return errors.New("[ERROR] CLASSROOM_ASSIGNMENT context requires contextId")
		}
		if r.Mode != model.AttemptModeAssignment {
			return errors.New("[ERROR] CLASSROOM_ASSIGNMENT attempts must omit mode")
		}
	default:
		return errors.New("[ERROR] Invalid context type")
	}

	seenParts := make(map[model.PartCode]struct{}, len(r.Parts))
	for _, part := range r.Parts {
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

func ValidateCreateExamAttemptRequestForExam(examType model.ExamType, req CreateExamAttemptRequest) error {
	if examType == model.ExamTypeAssignment {
		if req.Mode != model.AttemptModeAssignment {
			return errors.New("[ERROR] ASSIGNMENT exams require ASSIGNMENT mode")
		}
		if req.Section != nil {
			return errors.New("[ERROR] ASSIGNMENT mode does not support section")
		}
		if len(req.Parts) > 0 {
			return errors.New("[ERROR] ASSIGNMENT mode does not support parts")
		}
		if req.Duration != nil {
			return errors.New("[ERROR] ASSIGNMENT mode does not accept duration")
		}
		return nil
	}

	if req.Mode == model.AttemptModeAssignment {
		return errors.New("[ERROR] ASSIGNMENT mode only supports ASSIGNMENT exams")
	}

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

	return nil
}

type AttemptResponse struct {
	ID             uint                `json:"id"`
	ExamID         uint                `json:"examId"`
	Exam           *AttemptExamSummary `json:"exam,omitempty"`
	Mode           string              `json:"mode"`
	Flow           string              `json:"flow"`
	Stage          *string             `json:"stage,omitempty"`
	Status         string              `json:"status"`
	Section        *string             `json:"section,omitempty"`
	Parts          []string            `json:"parts,omitempty"`
	Duration       *int                `json:"duration,omitempty"`
	StartedAt      time.Time           `json:"startedAt"`
	ExpiresAt      *time.Time          `json:"expiresAt,omitempty"`
	SubmittedAt    *time.Time          `json:"submittedAt,omitempty"`
	TotalQuestions int                 `json:"totalQuestions"`
	AnsweredCount  int                 `json:"answeredCount"`
	CorrectAnswers *int                `json:"correctAnswers,omitempty"`
	Score          *float64            `json:"score,omitempty"`
	// Giám sát: chỉ có giá trị khi proctored.
	Proctored         bool                    `json:"proctored"`
	MaxViolations     *int                    `json:"maxViolations,omitempty"`
	ViolationCount    *int                    `json:"violationCount,omitempty"`
	IntegrityOutcome  *string                 `json:"integrityOutcome,omitempty"`
	TerminationReason *string                 `json:"terminationReason,omitempty"`
	CreatedAt         time.Time               `json:"createdAt"`
	UpdatedAt         time.Time               `json:"updatedAt"`
	Sections          []*ExamQuestionsSection `json:"sections,omitempty"`
}

type AttemptExamSummary struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Type        string  `json:"type"`
	Year        *int    `json:"year"`
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

	response := &AttemptResponse{
		ID:     attempt.ID,
		ExamID: attempt.ExamID,
		Exam:   exam,
		Mode:   string(attempt.Mode),
		Flow:   string(attemptFlow(attempt.Flow)),
		Stage: valuex.MapPtr(attempt.Stage, func(value model.AttemptStage) string {
			return string(value)
		}),
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
	applyIntegrityFields(response, attempt)
	return response
}

func applyIntegrityFields(response *AttemptResponse, attempt *model.Attempt) {
	if !attempt.Proctored {
		return
	}
	state := ToIntegrityState(attempt)
	response.Proctored = true
	response.MaxViolations = &state.MaxViolations
	response.ViolationCount = &state.ViolationCount
	response.IntegrityOutcome = &state.Outcome
	response.TerminationReason = terminationReason(attempt.TerminationReason)
}

// attemptFlow: attempt tạo trước khi có cột Flow (hoặc struct dựng tay) coi là FREE.
func attemptFlow(flow model.AttemptFlow) model.AttemptFlow {
	if flow == "" {
		return model.AttemptFlowFree
	}
	return flow
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
	questionsResp := &GetExamQuestionsResponse{Sections: ToAttemptSections(&attempt.Exam, attempt.SectionCode, attempt.PartCodes)}
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

// ToAttemptSections trả cây đề đã lọc theo section/parts của attempt, đã sắp thứ tự
// (section.order → part → group.order → question.order). Dùng cho response và cho layout.
func ToAttemptSections(exam *model.Exam, sectionCode *model.SectionCode, parts []model.PartCode) []*ExamQuestionsSection {
	query := GetExamQuestionsQuery{}
	if sectionCode != nil {
		query.Section = *sectionCode
	}
	return filterSectionsByParts(ToGetExamQuestionsResponse(exam, query).Sections, parts)
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

type SubmitAttemptRequest struct {
	Answers []SubmitAttemptAnswer `json:"answers" validate:"dive"`
}

type SubmitAttemptAnswer struct {
	QuestionID       uint  `json:"questionId" validate:"required,gt=0"`
	SelectedOptionID *uint `json:"selectedOptionId" validate:"omitempty,gt=0"`
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

type UpdateAttemptAnswerResponse struct {
	AttemptID        uint      `json:"attemptId"`
	QuestionID       uint      `json:"questionId"`
	SelectedOptionID *uint     `json:"selectedOptionId,omitempty"`
	AnsweredAt       time.Time `json:"answeredAt"`
}
