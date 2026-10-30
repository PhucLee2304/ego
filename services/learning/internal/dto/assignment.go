package dto

import (
	"encoding/json"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/model"
)

type AssignmentsQuery struct {
	httpx.PaginationQuery
}

type CreateAssignmentsRequest struct {
	Title       string                            `json:"title" validate:"required,notblank"`
	Description *string                           `json:"description,omitempty"`
	Questions   []CreateAssignmentQuestionRequest `json:"questions" validate:"required,min=1,dive"`
	Schedules   []AssignmentScheduleRequest       `json:"schedules" validate:"required,min=1,dive"`
}

type AssignmentScheduleRequest struct {
	ClassroomID uint      `json:"classroomId" validate:"required,gt=0"`
	Duration    int       `json:"duration" validate:"required,gt=0,lte=1440"`
	OpensAt     time.Time `json:"opensAt" validate:"required"`
	DueAt       time.Time `json:"dueAt" validate:"required"`
	// Giám sát (PLAN_ATTEMPT_INTEGRITY.md mục 1.1): MaxViolations mặc định 2.
	Proctored     bool `json:"proctored"`
	MaxViolations *int `json:"maxViolations,omitempty" validate:"omitempty,min=1"`
}

type UpdateAssignmentScheduleRequest struct {
	Duration int       `json:"duration" validate:"required,gt=0,lte=1440"`
	OpensAt  time.Time `json:"opensAt" validate:"required"`
	DueAt    time.Time `json:"dueAt" validate:"required"`
	Confirm  bool      `json:"confirm"`
	// Không gửi thì giữ nguyên; chỉ áp cho attempt tạo sau (Q4).
	Proctored     *bool `json:"proctored,omitempty"`
	MaxViolations *int  `json:"maxViolations,omitempty" validate:"omitempty,min=1"`
}

type CreateAssignmentQuestionRequest struct {
	Content            string   `json:"content" validate:"required,notblank"`
	Explanation        *string  `json:"explanation,omitempty"`
	Options            []string `json:"options" validate:"required,len=4,dive,notblank"`
	CorrectOptionIndex int      `json:"correctOptionIndex" validate:"gte=0,lte=3"`
	ExcludedFromScore  bool     `json:"excludedFromScore"`
}

type SetAssignmentActiveRequest struct {
	Active *bool `json:"active" validate:"required"`
}

type UpdateAssignmentContentRequest struct {
	Title       string                            `json:"title" validate:"required,notblank"`
	Description *string                           `json:"description,omitempty"`
	Questions   []UpdateAssignmentQuestionRequest `json:"questions" validate:"required,min=1,dive"`
	Confirm     bool                              `json:"confirm"`
	Reason      string                            `json:"reason" validate:"omitempty"`
}

// UpdateAssignmentRequest là màn sửa bài, cùng dạng với lúc tạo: toàn bộ nội dung đề và
// lịch của các lớp được tích. Backend tự so với dữ liệu hiện có: nội dung khác thì sửa đề,
// lớp đã có bài mà lịch khác thì sửa lịch, lớp chưa có thì giao thêm. Confirm/Reason dùng
// chung khi thay đổi ảnh hưởng tới bài đang làm.
type UpdateAssignmentRequest struct {
	Title       string                            `json:"title" validate:"required,notblank"`
	Description *string                           `json:"description,omitempty"`
	Questions   []UpdateAssignmentQuestionRequest `json:"questions" validate:"required,min=1,dive"`
	Schedules   []AssignmentScheduleRequest       `json:"schedules" validate:"omitempty,dive"`
	Confirm     bool                              `json:"confirm"`
	Reason      string                            `json:"reason" validate:"omitempty"`
}

type UpdateAssignmentQuestionRequest struct {
	ID                 uint     `json:"id" validate:"omitempty,gt=0"`
	Content            string   `json:"content" validate:"required,notblank"`
	Explanation        *string  `json:"explanation,omitempty"`
	Options            []string `json:"options" validate:"required,len=4,dive,notblank"`
	CorrectOptionIndex int      `json:"correctOptionIndex" validate:"gte=0,lte=3"`
	ExcludedFromScore  bool     `json:"excludedFromScore"`
	OptionIDs          []uint   `json:"optionIds" validate:"omitempty,len=4,dive,gt=0"`
}

type EditableAssignmentResponse struct {
	ID                  uint                          `json:"id"`
	ExamID              uint                          `json:"examId"`
	Title               string                        `json:"title"`
	Description         *string                       `json:"description,omitempty"`
	Questions           []*EditableAssignmentQuestion `json:"questions"`
	HasAttempts         bool                          `json:"hasAttempts"`
	ActiveAttempts      int64                         `json:"activeAttempts"`
	SubmittedAttempts   int64                         `json:"submittedAttempts"`
	AffectedAssignments []*AffectedAssignmentResponse `json:"affectedAssignments"`
}

type AffectedAssignmentResponse struct {
	ID            uint   `json:"id"`
	ClassroomID   uint   `json:"classroomId"`
	ClassroomName string `json:"classroomName"`
	Active        bool   `json:"active"`
	// Lịch của lớp đó, để màn sửa mở từ lớp nào cũng sửa được lịch mọi lớp.
	Duration      int       `json:"duration"`
	OpensAt       time.Time `json:"opensAt"`
	DueAt         time.Time `json:"dueAt"`
	Proctored     bool      `json:"proctored"`
	MaxViolations int       `json:"maxViolations"`
}

type EditableAssignmentQuestion struct {
	ID                 uint     `json:"id"`
	Content            string   `json:"content"`
	Explanation        *string  `json:"explanation,omitempty"`
	Options            []string `json:"options"`
	CorrectOptionIndex int      `json:"correctOptionIndex"`
	ExcludedFromScore  bool     `json:"excludedFromScore"`
	OptionIDs          []uint   `json:"optionIds"`
}

type ExamAuditResponse struct {
	ID           uint                            `json:"id"`
	ExamID       uint                            `json:"examId"`
	ActorID      string                          `json:"actorId"`
	Actor        *UserInfo                       `json:"actor,omitempty"`
	Reason       string                          `json:"reason"`
	Changes      []string                        `json:"changes"`
	Before       json.RawMessage                 `json:"before"`
	After        json.RawMessage                 `json:"after"`
	RegradedAt   *time.Time                      `json:"regradedAt,omitempty"`
	CreatedAt    time.Time                       `json:"createdAt"`
	ScoreChanges []*ExamAuditScoreChangeResponse `json:"scoreChanges"`
}

type ExamAuditScoreChangeResponse struct {
	AttemptID   uint     `json:"attemptId"`
	StudentID   string   `json:"studentId"`
	BeforeScore *float64 `json:"beforeScore"`
	AfterScore  *float64 `json:"afterScore"`
}

type AssignmentSubmissionResponse struct {
	ID           uint                   `json:"id,omitempty"`
	AssignmentID uint                   `json:"assignmentId"`
	Student      *UserInfo              `json:"student,omitempty"`
	AttemptID    uint                   `json:"attemptId,omitempty"`
	Score        *float64               `json:"score,omitempty"`
	Status       model.SubmissionStatus `json:"status"`
	SyncStatus   string                 `json:"syncStatus"`
	// Giám sát (chỉ có khi attempt proctored).
	Proctored         bool      `json:"proctored,omitempty"`
	ViolationCount    int       `json:"violationCount,omitempty"`
	PendingViolations int       `json:"pendingViolations,omitempty"`
	IntegrityOutcome  *string   `json:"integrityOutcome,omitempty"`
	TerminationReason *string   `json:"terminationReason,omitempty"`
	CreatedAt         time.Time `json:"createdAt,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt,omitempty"`
}

type AssignmentResponse struct {
	ID            uint                            `json:"id"`
	ClassroomID   uint                            `json:"classroomId"`
	Title         string                          `json:"title"`
	Description   *string                         `json:"description,omitempty"`
	ExamID        uint                            `json:"examId"`
	Duration      int                             `json:"duration"`
	OpensAt       time.Time                       `json:"opensAt"`
	DueAt         time.Time                       `json:"dueAt"`
	Active        bool                            `json:"active"`
	Proctored     bool                            `json:"proctored"`
	MaxViolations int                             `json:"maxViolations"`
	Submission    *AssignmentSubmissionResponse   `json:"submission,omitempty"`
	Submissions   []*AssignmentSubmissionResponse `json:"submissions,omitempty"`
	CreatedAt     time.Time                       `json:"createdAt"`
	UpdatedAt     time.Time                       `json:"updatedAt"`
}

type CreatedAssignmentResponse struct {
	ID            uint      `json:"id"`
	ClassroomID   uint      `json:"classroomId"`
	Title         string    `json:"title"`
	Description   *string   `json:"description,omitempty"`
	ExamID        uint      `json:"examId"`
	Duration      int       `json:"duration"`
	OpensAt       time.Time `json:"opensAt"`
	DueAt         time.Time `json:"dueAt"`
	Active        bool      `json:"active"`
	Proctored     bool      `json:"proctored"`
	MaxViolations int       `json:"maxViolations"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type CreatedAssignmentsResponse struct {
	Assignments []*CreatedAssignmentResponse `json:"assignments"`
}

func ToAssignmentResponse(assignment *model.Assignment, now time.Time, currentUserID string, includeSubmissions bool, users []*identityClient.UserInfo) *AssignmentResponse {
	response := &AssignmentResponse{
		ID:            assignment.ID,
		ClassroomID:   assignment.ClassroomID,
		Title:         assignment.Title,
		Description:   assignment.Description,
		ExamID:        assignment.ExamID,
		Duration:      assignment.Duration,
		OpensAt:       assignment.OpensAt,
		DueAt:         assignment.DueAt,
		Active:        assignment.Active,
		Proctored:     assignment.Proctored,
		MaxViolations: assignment.MaxViolations,
		CreatedAt:     assignment.CreatedAt,
		UpdatedAt:     assignment.UpdatedAt,
	}

	usersByID := ToUserInfoMap(users)
	for i := range assignment.Submissions {
		submission := ToAssignmentSubmissionResponse(&assignment.Submissions[i], now, assignment.DueAt)
		submission.Student = usersByID[assignment.Submissions[i].StudentID]
		if includeSubmissions {
			response.Submissions = append(response.Submissions, submission)
		}
		if assignment.Submissions[i].StudentID == currentUserID {
			response.Submission = submission
		}
	}

	if currentUserID != "" && response.Submission == nil {
		response.Submission = &AssignmentSubmissionResponse{
			AssignmentID: assignment.ID,
			Status:       model.Status(nil, now, assignment.DueAt),
		}
	}

	return response
}

func ToAssignmentResponses(assignments []*model.Assignment, now time.Time, currentUserID string, includeSubmissions bool, users []*identityClient.UserInfo) []*AssignmentResponse {
	responses := make([]*AssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = ToAssignmentResponse(assignment, now, currentUserID, includeSubmissions, users)
	}
	return responses
}

func ToCreatedAssignmentResponse(assignment *model.Assignment) *CreatedAssignmentResponse {
	return &CreatedAssignmentResponse{
		ID:            assignment.ID,
		ClassroomID:   assignment.ClassroomID,
		Title:         assignment.Title,
		Description:   assignment.Description,
		ExamID:        assignment.ExamID,
		Duration:      assignment.Duration,
		OpensAt:       assignment.OpensAt,
		DueAt:         assignment.DueAt,
		Active:        assignment.Active,
		Proctored:     assignment.Proctored,
		MaxViolations: assignment.MaxViolations,
		CreatedAt:     assignment.CreatedAt,
		UpdatedAt:     assignment.UpdatedAt,
	}
}

func ToCreatedAssignmentsResponse(assignments []*model.Assignment) *CreatedAssignmentsResponse {
	responses := make([]*CreatedAssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = ToCreatedAssignmentResponse(assignment)
	}
	return &CreatedAssignmentsResponse{Assignments: responses}
}

func ToAssignmentSubmissionResponse(submission *model.AssignmentSubmission, now time.Time, dueAt time.Time) *AssignmentSubmissionResponse {
	syncStatus := "NOT_STARTED"
	if submission.AttemptID > 0 {
		syncStatus = "PENDING"
	}
	if submission.Score != nil {
		syncStatus = "SYNCED"
	}
	return &AssignmentSubmissionResponse{
		ID:           submission.ID,
		AssignmentID: submission.AssignmentID,
		AttemptID:    submission.AttemptID,
		Score:        submission.Score,
		Status:       model.Status(submission, now, dueAt),
		SyncStatus:   syncStatus,
		CreatedAt:    submission.CreatedAt,
		UpdatedAt:    submission.UpdatedAt,
	}
}

// ApplySubmissionIntegrity gắn tóm tắt giám sát của attempt vào dòng gradebook.
func ApplySubmissionIntegrity(response *AssignmentSubmissionResponse, summary *model.AttemptIntegritySummary) {
	if summary == nil || !summary.Proctored {
		return
	}
	outcome := string(summary.IntegrityOutcome)
	response.Proctored = true
	response.ViolationCount = summary.ViolationCount
	response.PendingViolations = summary.PendingViolations
	response.IntegrityOutcome = &outcome
	response.TerminationReason = terminationReason(summary.TerminationReason)
}

// ToEditableAssignmentQuestions chuyển đề private sang danh sách câu hỏi để sửa
// (giữ đúng cách map của PrivateExamResponse bên exams cũ).
func ToEditableAssignmentQuestions(exam *model.Exam) []*EditableAssignmentQuestion {
	questions := make([]*EditableAssignmentQuestion, 0)
	for _, section := range exam.Sections {
		for _, question := range section.Questions {
			item := &EditableAssignmentQuestion{ID: question.ID, Content: question.Content, ExcludedFromScore: question.ExcludedFromScore, Options: []string{}, OptionIDs: []uint{}}
			if question.Explanation != "" {
				explanation := question.Explanation
				item.Explanation = &explanation
			}
			for index, option := range question.Options {
				item.OptionIDs = append(item.OptionIDs, option.ID)
				if option.Content != nil {
					item.Options = append(item.Options, *option.Content)
				} else {
					item.Options = append(item.Options, "")
				}
				if option.IsCorrect {
					item.CorrectOptionIndex = index
				}
			}
			questions = append(questions, item)
		}
	}
	return questions
}

// ToExamAuditResponse giữ đúng dữ liệu cũ đi qua gRPC: thời gian làm tròn tới giây.
func ToExamAuditResponse(audit *model.ExamAudit) (*ExamAuditResponse, error) {
	item := &ExamAuditResponse{ID: audit.ID, ExamID: audit.ExamID, ActorID: audit.ActorID, Reason: audit.Reason, Before: json.RawMessage(audit.Before), After: json.RawMessage(audit.After), CreatedAt: time.Unix(audit.CreatedAt.Unix(), 0), ScoreChanges: []*ExamAuditScoreChangeResponse{}, Changes: []string{}}
	if len(audit.Changes) > 0 {
		if err := json.Unmarshal(audit.Changes, &item.Changes); err != nil {
			return nil, err
		}
	}
	if audit.RegradedAt != nil {
		value := time.Unix(audit.RegradedAt.Unix(), 0)
		item.RegradedAt = &value
	}
	var changes []model.ExamAuditScoreChange
	if len(audit.ScoreChanges) > 0 {
		if err := json.Unmarshal(audit.ScoreChanges, &changes); err != nil {
			return nil, err
		}
	}
	for _, change := range changes {
		item.ScoreChanges = append(item.ScoreChanges, &ExamAuditScoreChangeResponse{AttemptID: change.AttemptID, StudentID: change.StudentID, BeforeScore: change.BeforeScore, AfterScore: change.AfterScore})
	}
	return item, nil
}
