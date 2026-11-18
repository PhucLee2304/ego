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
	// Tùy chọn: bỏ attempt theo mode (vd ASSIGNMENT cho lịch sử tự luyện).
	ExcludeMode model.AttemptMode `schema:"excludeMode" validate:"omitempty,oneof=PRACTICE TEST ASSIGNMENT"`
	// Tùy chọn: chỉ lấy attempt của đề thuộc các loại này, phân cách bằng dấu phẩy (vd "TOEIC,THPT").
	ExamType string `schema:"examType"`
	// Tùy chọn: tìm theo tên đề (không phân biệt hoa thường).
	Search string `schema:"search"`
}

func (q *GetAttemptsQuery) Normalize() {
	q.PaginationQuery.Normalize()
	q.Status = model.AttemptStatus(strings.ToUpper(strings.TrimSpace(string(q.Status))))
	q.ExcludeMode = model.AttemptMode(strings.ToUpper(strings.TrimSpace(string(q.ExcludeMode))))
	q.ExamType = strings.ToUpper(strings.TrimSpace(q.ExamType))
	q.Search = strings.TrimSpace(q.Search)
}

// ExamTypes tách ExamType thành danh sách, bỏ phần tử rỗng.
func (q GetAttemptsQuery) ExamTypes() []model.ExamType {
	var types []model.ExamType
	for _, t := range strings.Split(q.ExamType, ",") {
		if t = strings.TrimSpace(t); t != "" {
			types = append(types, model.ExamType(t))
		}
	}
	return types
}

// CreateExamAttemptRequest là body tạo lượt làm bài cho một đề.
//   - ContextType: bắt buộc. STANDALONE là tự làm đề; CLASSROOM_ASSIGNMENT là làm bài tập lớp.
//   - ContextID: ID assignment, bắt buộc với CLASSROOM_ASSIGNMENT và không được gửi với STANDALONE.
//   - Mode: PRACTICE hoặc TEST với STANDALONE; bỏ trống với CLASSROOM_ASSIGNMENT (tự thành ASSIGNMENT).
//   - Section, Parts: giới hạn phạm vi đề, chỉ dùng một trong hai. Bỏ trống cả hai là làm cả đề.
//   - Duration: thời gian tự chọn (phút), chỉ có tác dụng với PRACTICE; bỏ trống là không giới hạn.
//     TEST dùng thời gian chuẩn theo loại đề, ASSIGNMENT dùng thời gian của bài tập.
//   - IntegrityConsent: bắt buộc khi assignment có giám sát; version phải khớp phiên bản điều khoản hiện tại.
type CreateExamAttemptRequest struct {
	Mode             model.AttemptMode        `json:"mode,omitempty" validate:"omitempty,oneof=PRACTICE TEST ASSIGNMENT"`
	Section          *model.SectionCode       `json:"section,omitempty" validate:"omitempty,oneof=FULL LISTENING READING"`
	Parts            []model.PartCode         `json:"parts,omitempty" validate:"omitempty,dive,oneof=1 2 3 4 5 6 7"`
	Duration         *model.AttemptDuration   `json:"duration,omitempty" validate:"omitempty,oneof=10 15 20 30 45 60 75 90 120"`
	ContextType      model.AttemptContextType `json:"contextType" validate:"required,oneof=STANDALONE CLASSROOM_ASSIGNMENT"`
	ContextID        *uint                    `json:"contextId,omitempty" validate:"omitempty,gt=0"`
	IntegrityConsent *IntegrityConsentRequest `json:"integrityConsent,omitempty"`
}

// Normalize chuẩn hóa body trước khi validate: đưa Mode, ContextType, Section về chữ hoa,
// bỏ khoảng trắng thừa ở Parts. Bài tập lớp không gửi Mode thì tự đặt là ASSIGNMENT.
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

// Validate kiểm các ràng buộc giữa các field mà tag validate không diễn tả được:
//   - STANDALONE: Mode phải là PRACTICE/TEST, không được có ContextID.
//   - CLASSROOM_ASSIGNMENT: phải có ContextID, Mode phải bỏ trống (đã thành ASSIGNMENT ở Normalize).
//   - Parts không được trùng, và không dùng chung Section với Parts.
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

// ValidateCreateExamAttemptRequestForExam kiểm các ràng buộc phụ thuộc loại đề, chạy sau
// khi đã tải đề vì Validate của request chưa biết loại đề:
//   - Đề ASSIGNMENT chỉ làm ở chế độ ASSIGNMENT, luôn làm cả đề với thời gian của bài tập.
//   - Chế độ TEST làm cả đề với thời gian chuẩn, để giống thi thật.
//   - Luyện tập: chỉ TOEIC chọn được part; THPT chỉ có FULL; TOEIC chọn LISTENING hoặc READING.
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
	Proctored         bool    `json:"proctored"`
	MaxViolations     *int    `json:"maxViolations,omitempty"`
	ViolationCount    *int    `json:"violationCount,omitempty"`
	IntegrityOutcome  *string `json:"integrityOutcome,omitempty"`
	TerminationReason *string `json:"terminationReason,omitempty"`
	// TeacherNote: lời nhắn giáo viên cho học sinh khi đình chỉ / mở lại.
	TeacherNote *string `json:"teacherNote,omitempty"`
	// Bài giao trong lớp: client mở kết quả trong tab Classwork của lớp đó.
	AssignmentID *uint                   `json:"assignmentId,omitempty"`
	ClassroomID  *uint                   `json:"classroomId,omitempty"`
	CreatedAt    time.Time               `json:"createdAt"`
	UpdatedAt    time.Time               `json:"updatedAt"`
	Sections     []*ExamQuestionsSection `json:"sections,omitempty"`
}

type AttemptExamSummary struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Type        string  `json:"type"`
	Year        *int    `json:"year"`
}

// ToAttemptResponse dựng thông tin lượt làm bài không kèm câu hỏi (trạng thái, thời gian,
// phạm vi, luồng làm bài, kết quả, giám sát). Dùng khi client lấy nội dung theo trang
// (TOEIC_STRICT) hoặc chỉ cần danh sách lượt làm bài. Tham số now hiện chưa được dùng.
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
		AssignmentID:   assignmentID(attempt),
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

// ToAttemptResponseWithQuestions dựng lượt làm bài kèm câu hỏi trong phạm vi và đáp án đã
// lưu, dùng cho luồng FREE. Không kèm kết quả chấm; lời giải bị ẩn khi bài còn đang làm.
func ToAttemptResponseWithQuestions(attempt *model.Attempt, now time.Time) *AttemptResponse {
	return toAttemptResponseWithQuestions(attempt, false, now)
}

func ToAttemptHistoryResponse(attempt *model.Attempt) *AttemptResponse {
	// Bài đã nộp: thời gian còn lại không còn ý nghĩa nên dùng giờ thật.
	return toAttemptResponseWithQuestions(attempt, attempt.Status == model.AttemptStatusSubmitted, time.Now())
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

// toAttemptResponseWithQuestions dựng lượt làm bài kèm cây câu hỏi trong phạm vi của lượt và đáp
// án học viên đã chọn. Là phần chung của hai trường hợp:
//   - Đang làm (ToAttemptResponseWithQuestions, includeResults = false): chỉ kèm đáp án đã chọn,
//     ẩn transcript và lời giải để không lộ đáp án.
//   - Bản chụp lịch sử (ToAttemptHistoryResponse): kèm thêm đúng/sai và đáp án đúng của từng câu
//     khi bài đã nộp (includeResults = true); bài hủy thì không kèm.
//
// Cờ đúng/sai trên từng option không bao giờ ra JSON, nên chỉ lộ đáp án qua CorrectOptionID khi
// includeResults bật.
func toAttemptResponseWithQuestions(attempt *model.Attempt, includeResults bool, now time.Time) *AttemptResponse {
	// Chỉ lấy các câu thuộc phạm vi đã chốt của lượt (section/part), theo thứ tự hiển thị.
	questionsResp := &GetExamQuestionsResponse{Sections: ToAttemptSections(&attempt.Exam, attempt.SectionCode, attempt.PartCodes)}
	// Bài còn đang làm thì ẩn transcript và lời giải; xem lại sau khi nộp/hủy thì được thấy.
	hideSolutions := attempt.Status == model.AttemptStatusInProgress
	// Tra đáp án đã lưu theo câu để gắn vào từng câu bên dưới.
	answers := make(map[uint]model.AttemptAnswer, len(attempt.Answers))
	for _, answer := range attempt.Answers {
		answers[answer.QuestionID] = answer
	}

	// Duyệt cả câu trong group lẫn câu đứng riêng: ẩn lời giải nếu cần và gắn đáp án của câu.
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

	// Phần thông tin lượt làm bài (trạng thái, thời gian, điểm, giám sát) dùng chung với
	// ToAttemptResponse, chỉ gắn thêm cây câu hỏi.
	resp := ToAttemptResponse(attempt, now)
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

// applyAttemptAnswer gắn đáp án học viên đã chọn vào câu. includeResults bật thì gắn thêm câu đó
// đúng hay sai và ID của đáp án đúng, để màn xem lại tô màu được.
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
	// SessionToken: token nhận từ ATTEMPT_SESSION_CLAIM, bắt buộc với bài có giám sát đã claim.
	SessionToken string `json:"sessionToken,omitempty"`
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

func assignmentID(attempt *model.Attempt) *uint {
	if attempt.ContextType != model.AttemptContextTypeClassroomAssignment {
		return nil
	}
	return attempt.ContextID
}
