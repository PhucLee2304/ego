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

type CreateClassroomRequest struct {
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
