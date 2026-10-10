package dto

import (
	"time"

	usersClient "ego/api/gen/go/users"
	"ego/platform/httpx"
	"ego/services/classrooms/internal/model"
)

type ClassroomsQuery struct {
	httpx.PaginationQuery
	Query string `schema:"query" validate:"omitempty"`
}

type GetClassroomByCodeQuery struct {
	Code string `schema:"code" validate:"required,notblank"`
}

type CreateClassroomRequest struct {
	// Product decision: names and descriptions use PostgreSQL TEXT without an application-level length limit.
	Name        string  `json:"name" validate:"required,notblank"`
	Description *string `json:"description,omitempty"`
	Avatar      *string `json:"avatar,omitempty" validate:"omitempty,notblank"`
}

type UpdateClassroomRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,notblank"`
	Description *string `json:"description,omitempty"`
	Avatar      *string `json:"avatar,omitempty" validate:"omitempty,notblank"`
}

type RequestJoinClassroomRequest struct {
	Code string `json:"code" validate:"required,notblank"`
}

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
}

type AssignExistingExamRequest struct {
	Schedules []AssignmentScheduleRequest `json:"schedules" validate:"required,min=1,dive"`
}

type UpdateAssignmentScheduleRequest struct {
	Duration int       `json:"duration" validate:"required,gt=0,lte=1440"`
	OpensAt  time.Time `json:"opensAt" validate:"required"`
	DueAt    time.Time `json:"dueAt" validate:"required"`
	Confirm  bool      `json:"confirm"`
}

type CreateAssignmentQuestionRequest struct {
	Content            string   `json:"content" validate:"required,notblank"`
	Explanation        *string  `json:"explanation,omitempty"`
	Options            []string `json:"options" validate:"required,len=4,dive,notblank"`
	CorrectOptionIndex int      `json:"correctOptionIndex" validate:"gte=0,lte=3"`
}

type UpdateAssignmentContentRequest struct {
	Title       string                            `json:"title" validate:"required,notblank"`
	Description *string                           `json:"description,omitempty"`
	Questions   []UpdateAssignmentQuestionRequest `json:"questions" validate:"required,min=1,dive"`
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

type SetAssignmentActiveRequest struct {
	Active *bool `json:"active" validate:"required"`
}

type ClassroomResponse struct {
	ID          uint                            `json:"id"`
	Code        string                          `json:"code"`
	Name        string                          `json:"name"`
	Description *string                         `json:"description,omitempty"`
	Avatar      *string                         `json:"avatar,omitempty"`
	Teacher     *UserInfo                       `json:"teacher,omitempty"`
	Members     []*ClassroomJoinRequestResponse `json:"members,omitempty"`
	Active      bool                            `json:"active"`
	CreatedAt   time.Time                       `json:"createdAt"`
	UpdatedAt   time.Time                       `json:"updatedAt"`
}

type CreatedClassroomResponse struct {
	ID          uint      `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	Avatar      *string   `json:"avatar,omitempty"`
	Teacher     *UserInfo `json:"teacher,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ClassroomByCodeResponse struct {
	ID                   uint      `json:"id"`
	Code                 string    `json:"code"`
	Name                 string    `json:"name"`
	Description          *string   `json:"description,omitempty"`
	Avatar               *string   `json:"avatar,omitempty"`
	Teacher              *UserInfo `json:"teacher,omitempty"`
	CountApprovedMembers int64     `json:"countApprovedMembers"`
	Active               bool      `json:"active"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type UserInfo struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func ToUserInfoMap(users []*usersClient.UserInfo) map[string]*UserInfo {
	usersByID := make(map[string]*UserInfo, len(users))
	for _, user := range users {
		usersByID[user.Id] = &UserInfo{
			ID:     user.Id,
			Email:  user.Email,
			Name:   user.Name,
			Avatar: user.Avatar,
		}
	}
	return usersByID
}

type ClassroomSummaryResponse struct {
	ID      uint      `json:"id"`
	Code    string    `json:"code"`
	Name    string    `json:"name"`
	Avatar  *string   `json:"avatar,omitempty"`
	Teacher *UserInfo `json:"teacher,omitempty"`
	Active  bool      `json:"active"`
}

type ClassroomJoinRequestResponse struct {
	ID         uint                      `json:"id"`
	User       *UserInfo                 `json:"user,omitempty"`
	Classroom  *ClassroomSummaryResponse `json:"classroom,omitempty"`
	Approved   *bool                     `json:"approved"`
	ApprovedAt *time.Time                `json:"approvedAt,omitempty"`
	JoinedAt   *time.Time                `json:"joinedAt,omitempty"`
	CreatedAt  time.Time                 `json:"createdAt"`
	UpdatedAt  time.Time                 `json:"updatedAt"`
}

type AssignmentSubmissionResponse struct {
	ID           uint                   `json:"id,omitempty"`
	AssignmentID uint                   `json:"assignmentId"`
	Student      *UserInfo              `json:"student,omitempty"`
	AttemptID    uint                   `json:"attemptId,omitempty"`
	Score        *float64               `json:"score,omitempty"`
	Status       model.SubmissionStatus `json:"status"`
	SyncStatus   string                 `json:"syncStatus"`
	CreatedAt    time.Time              `json:"createdAt,omitempty"`
	UpdatedAt    time.Time              `json:"updatedAt,omitempty"`
}

type AssignmentResponse struct {
	ID          uint                            `json:"id"`
	ClassroomID uint                            `json:"classroomId"`
	Title       string                          `json:"title"`
	Description *string                         `json:"description,omitempty"`
	ExamID      uint                            `json:"examId"`
	Duration    int                             `json:"duration"`
	OpensAt     time.Time                       `json:"opensAt"`
	DueAt       time.Time                       `json:"dueAt"`
	Active      bool                            `json:"active"`
	Submission  *AssignmentSubmissionResponse   `json:"submission,omitempty"`
	Submissions []*AssignmentSubmissionResponse `json:"submissions,omitempty"`
	CreatedAt   time.Time                       `json:"createdAt"`
	UpdatedAt   time.Time                       `json:"updatedAt"`
}

type CreatedAssignmentResponse struct {
	ID          uint      `json:"id"`
	ClassroomID uint      `json:"classroomId"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	ExamID      uint      `json:"examId"`
	Duration    int       `json:"duration"`
	OpensAt     time.Time `json:"opensAt"`
	DueAt       time.Time `json:"dueAt"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CreatedAssignmentsResponse struct {
	Assignments []*CreatedAssignmentResponse `json:"assignments"`
}

func ToClassroomResponse(classroom *model.Classroom) *ClassroomResponse {
	members := make([]*ClassroomJoinRequestResponse, len(classroom.Members))
	for i := range classroom.Members {
		members[i] = ToClassroomJoinRequestResponse(&classroom.Members[i])
	}

	return &ClassroomResponse{
		ID:          classroom.ID,
		Code:        classroom.Code,
		Name:        classroom.Name,
		Description: classroom.Description,
		Avatar:      classroom.Avatar,
		Members:     members,
		Active:      classroom.Active,
		CreatedAt:   classroom.CreatedAt,
		UpdatedAt:   classroom.UpdatedAt,
	}
}

func ToAssignmentResponse(assignment *model.Assignment, now time.Time, currentUserID string, includeSubmissions bool, users []*usersClient.UserInfo) *AssignmentResponse {
	response := &AssignmentResponse{
		ID:          assignment.ID,
		ClassroomID: assignment.ClassroomID,
		Title:       assignment.Title,
		Description: assignment.Description,
		ExamID:      assignment.ExamID,
		Duration:    assignment.Duration,
		OpensAt:     assignment.OpensAt,
		DueAt:       assignment.DueAt,
		Active:      assignment.Active,
		CreatedAt:   assignment.CreatedAt,
		UpdatedAt:   assignment.UpdatedAt,
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

func ToAssignmentResponses(assignments []*model.Assignment, now time.Time, currentUserID string, includeSubmissions bool, users []*usersClient.UserInfo) []*AssignmentResponse {
	responses := make([]*AssignmentResponse, len(assignments))
	for i, assignment := range assignments {
		responses[i] = ToAssignmentResponse(assignment, now, currentUserID, includeSubmissions, users)
	}
	return responses
}

func ToCreatedAssignmentResponse(assignment *model.Assignment) *CreatedAssignmentResponse {
	return &CreatedAssignmentResponse{
		ID:          assignment.ID,
		ClassroomID: assignment.ClassroomID,
		Title:       assignment.Title,
		Description: assignment.Description,
		ExamID:      assignment.ExamID,
		Duration:    assignment.Duration,
		OpensAt:     assignment.OpensAt,
		DueAt:       assignment.DueAt,
		Active:      assignment.Active,
		CreatedAt:   assignment.CreatedAt,
		UpdatedAt:   assignment.UpdatedAt,
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

func ToClassroomResponses(classrooms []*model.Classroom, users []*usersClient.UserInfo) []*ClassroomResponse {
	responses := make([]*ClassroomResponse, len(classrooms))
	usersByID := ToUserInfoMap(users)
	for i, classroom := range classrooms {
		responses[i] = ToClassroomResponse(classroom)
		responses[i].Teacher = usersByID[classroom.TeacherID]
		for j := range classroom.Members {
			responses[i].Members[j].User = usersByID[classroom.Members[j].UserID]
		}
	}
	return responses
}

func ToCreatedClassroomResponse(classroom *model.Classroom) *CreatedClassroomResponse {
	return &CreatedClassroomResponse{
		ID:          classroom.ID,
		Code:        classroom.Code,
		Name:        classroom.Name,
		Description: classroom.Description,
		Avatar:      classroom.Avatar,
		Active:      classroom.Active,
		CreatedAt:   classroom.CreatedAt,
		UpdatedAt:   classroom.UpdatedAt,
	}
}

func ToClassroomByCodeResponse(classroom *model.Classroom, countApprovedMembers int64) *ClassroomByCodeResponse {
	return &ClassroomByCodeResponse{
		ID:                   classroom.ID,
		Code:                 classroom.Code,
		Name:                 classroom.Name,
		Description:          classroom.Description,
		Avatar:               classroom.Avatar,
		CountApprovedMembers: countApprovedMembers,
		Active:               classroom.Active,
		CreatedAt:            classroom.CreatedAt,
		UpdatedAt:            classroom.UpdatedAt,
	}
}

func ToClassroomSummaryResponse(classroom *model.Classroom) *ClassroomSummaryResponse {
	return &ClassroomSummaryResponse{
		ID:     classroom.ID,
		Code:   classroom.Code,
		Name:   classroom.Name,
		Avatar: classroom.Avatar,
		Active: classroom.Active,
	}
}

func ToClassroomJoinRequestResponse(member *model.Member) *ClassroomJoinRequestResponse {
	response := &ClassroomJoinRequestResponse{
		ID:         member.ID,
		Approved:   member.Approved,
		ApprovedAt: member.ApprovedAt,
		JoinedAt:   member.JoinedAt,
		CreatedAt:  member.CreatedAt,
		UpdatedAt:  member.UpdatedAt,
	}
	if member.Classroom.ID != 0 {
		response.Classroom = ToClassroomSummaryResponse(&member.Classroom)
	}
	return response
}
