package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

func (s *service) CreateSchedule(ctx context.Context, teacherID string, classroomID uint, req dto.CreateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !req.EndsAt.After(req.StartsAt) {
		return nil, ErrScheduleInvalidTime
	}
	schedule := &model.Schedule{ClassroomID: classroomID, Title: strings.TrimSpace(req.Title), Description: req.Description, StartsAt: req.StartsAt, EndsAt: req.EndsAt}
	if schedule.Description != nil {
		value := strings.TrimSpace(*schedule.Description)
		schedule.Description = &value
	}
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		if err := repo.CreateSchedule(ctx, schedule); err != nil {
			return err
		}
		room := &model.Room{ScheduleID: schedule.ID, Status: model.RoomStatusScheduled, Schedule: *schedule}
		if err := repo.CreateRoom(ctx, room); err != nil {
			return err
		}
		schedule.Room = room
		return nil
	}); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToCalendarResponse(schedule), nil
}

func (s *service) GetCalendar(ctx context.Context, userID string, classroomID uint, query dto.CalendarQuery) ([]*dto.CalendarResponse, *httpx.HTTPError) {
	if !query.To.After(query.From) || query.To.Sub(query.From) > 366*24*time.Hour {
		return nil, ErrCalendarInvalidRange
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != userID {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, errClassroomServiceFailed)
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	schedules, err := s.repo.GetSchedulesByRange(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	assignments, err := s.repo.GetAssignmentsDueInRange(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	responses := make([]*dto.CalendarResponse, 0, len(schedules)+len(assignments))
	for _, schedule := range schedules {
		responses = append(responses, dto.ToCalendarResponse(schedule))
	}
	for _, assignment := range assignments {
		assignmentID := assignment.ID
		responses = append(responses, &dto.CalendarResponse{ClassroomID: classroomID, CalendarType: dto.CalendarTypeAssignmentDeadline, Title: "Deadline: " + assignment.Title, Description: assignment.Description, StartsAt: assignment.DueAt, RelatedAssignmentID: &assignmentID, Editable: false})
	}
	sort.SliceStable(responses, func(i, j int) bool {
		if responses[i].StartsAt.Equal(responses[j].StartsAt) {
			return responses[i].ID < responses[j].ID
		}
		return responses[i].StartsAt.Before(responses[j].StartsAt)
	})
	return responses, nil
}

func (s *service) UpdateSchedule(ctx context.Context, teacherID string, scheduleID uint, req dto.UpdateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError) {
	schedule, err := s.repo.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
	if err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !req.EndsAt.After(req.StartsAt) {
		return nil, ErrScheduleInvalidTime
	}
	schedule.Title, schedule.Description = strings.TrimSpace(req.Title), req.Description
	schedule.StartsAt, schedule.EndsAt = req.StartsAt, req.EndsAt
	if schedule.Description != nil {
		value := strings.TrimSpace(*schedule.Description)
		schedule.Description = &value
	}
	if err := s.repo.UpdateSchedule(ctx, schedule); err != nil {
		return nil, httpx.FromError(err, errClassroomServiceFailed)
	}
	return dto.ToCalendarResponse(schedule), nil
}

func (s *service) DeleteSchedule(ctx context.Context, teacherID string, scheduleID uint) *httpx.HTTPError {
	schedule, err := s.repo.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
	if err != nil {
		return httpx.FromError(err, errClassroomServiceFailed)
	}
	if classroom.TeacherID != teacherID {
		return ErrClassroomForbidden
	}
	if !classroom.Active {
		return ErrClassroomNotActive
	}
	if schedule.Room != nil && schedule.Room.Status == model.RoomStatusLive {
		return ErrRoomLive
	}
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		if schedule.Room != nil {
			if err := repo.DeleteRoom(ctx, schedule.Room); err != nil {
				return err
			}
		}
		return repo.DeleteSchedule(ctx, schedule)
	})
	return httpx.FromError(err, errClassroomServiceFailed)
}
