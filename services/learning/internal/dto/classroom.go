package dto

import (
	"strings"
	"time"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"
	"ego/services/learning/internal/model"
)

type ClassroomsQuery struct {
	httpx.PaginationQuery
	Query  string `schema:"query" validate:"omitempty,notblank"`
	Active *bool  `schema:"active" validate:"omitempty"`
}

func (q *ClassroomsQuery) Normalize() {
	q.PaginationQuery.Normalize()
	q.Query = strings.TrimSpace(q.Query)
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

func ToClassroomResponses(classrooms []*model.Classroom, users []*identityClient.UserInfo) []*ClassroomResponse {
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
