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

// CreateSchedule tạo buổi học trên lịch của lớp, kèm luôn phòng học trực tuyến ở trạng thái
// SCHEDULED trong cùng transaction, vì mỗi buổi học có đúng một phòng.
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
	if startsInPast(req.StartsAt) {
		return nil, ErrScheduleInPast
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

// GetCalendar trả lịch của lớp trong khoảng [From, To] (tối đa khoảng một năm để giới hạn tải):
// gồm các buổi học và hạn nộp bài tập, sắp theo thời gian để client vẽ một dòng thời gian.
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

// UpdateSchedule cho giáo viên đổi tiêu đề, mô tả, thời gian của buổi học. Không cho đặt giờ
// bắt đầu đã qua, như khi tạo.
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
	if startsInPast(req.StartsAt) {
		return nil, ErrScheduleInPast
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

// DeleteSchedule xóa buổi học cùng phòng của nó. Không xóa được khi phòng đang LIVE, để
// không cắt ngang buổi học đang diễn ra.
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

// schedulePastTolerance bù lệch đồng hồ giữa client và server khi chặn lịch bắt đầu trong quá khứ.
const schedulePastTolerance = 5 * time.Minute

// startsInPast cho biết giờ bắt đầu đã qua, có dung sai schedulePastTolerance để chênh lệch
// đồng hồ giữa client và server không làm hỏng việc đặt lịch sát giờ.
func startsInPast(startsAt time.Time) bool {
	return startsAt.Before(time.Now().Add(-schedulePastTolerance))
}
