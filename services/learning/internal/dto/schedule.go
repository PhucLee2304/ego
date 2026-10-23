package dto

import (
	"time"

	"ego/services/learning/internal/model"
)

type CalendarQuery struct {
	From time.Time `schema:"from" validate:"required"`
	To   time.Time `schema:"to" validate:"required"`
}

type CreateScheduleRequest struct {
	Title       string    `json:"title" validate:"required,notblank"`
	Description *string   `json:"description,omitempty"`
	StartsAt    time.Time `json:"startsAt" validate:"required"`
	EndsAt      time.Time `json:"endsAt" validate:"required"`
}

type UpdateScheduleRequest struct {
	Title       string    `json:"title" validate:"required,notblank"`
	Description *string   `json:"description,omitempty"`
	StartsAt    time.Time `json:"startsAt" validate:"required"`
	EndsAt      time.Time `json:"endsAt" validate:"required"`
}

type CalendarType string

const (
	CalendarTypeClassSchedule      CalendarType = "CLASS_SCHEDULE"
	CalendarTypeAssignmentDeadline CalendarType = "ASSIGNMENT_DEADLINE"
)

type CalendarResponse struct {
	ID                  uint          `json:"id"`
	ClassroomID         uint          `json:"classroomId"`
	CalendarType        CalendarType  `json:"calendarType"`
	Title               string        `json:"title"`
	Description         *string       `json:"description,omitempty"`
	StartsAt            time.Time     `json:"startsAt"`
	EndsAt              *time.Time    `json:"endsAt,omitempty"`
	RelatedAssignmentID *uint         `json:"relatedAssignmentId,omitempty"`
	Editable            bool          `json:"editable"`
	CreatedAt           *time.Time    `json:"createdAt,omitempty"`
	UpdatedAt           *time.Time    `json:"updatedAt,omitempty"`
	Room                *RoomResponse `json:"room,omitempty"`
}

func ToCalendarResponse(schedule *model.Schedule) *CalendarResponse {
	response := &CalendarResponse{
		ID:           schedule.ID,
		ClassroomID:  schedule.ClassroomID,
		CalendarType: CalendarTypeClassSchedule,
		Title:        schedule.Title,
		Description:  schedule.Description,
		StartsAt:     schedule.StartsAt,
		EndsAt:       &schedule.EndsAt,
		Editable:     true,
		CreatedAt:    &schedule.CreatedAt,
		UpdatedAt:    &schedule.UpdatedAt,
	}
	if schedule.Room != nil {
		// Preload("Room") không gắn ngược Schedule vào room → gắn tay để room có startsAt/endsAt.
		room := *schedule.Room
		room.Schedule = *schedule
		response.Room = ToRoomResponse(&room, false)
	}
	return response
}
