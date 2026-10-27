package dto

import (
	"time"

	"ego/services/learning/internal/model"
)

type RequestJoinClassroomRequest struct {
	Code string `json:"code" validate:"required,notblank"`
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
