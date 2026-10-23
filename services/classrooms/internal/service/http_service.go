package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/classrooms/internal/dto"
)

// HTTPService adapts classroom domain errors for HTTP handlers without
// coupling RPC, socket, webhook, and worker callers to HTTP status codes.
type HTTPService struct {
	service Service
}

func NewHTTPService(service Service) *HTTPService {
	return &HTTPService{service: service}
}

func (h *HTTPService) CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError) {
	response, err := h.service.CreateClassroom(ctx, teacherID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError) {
	response, count, err := h.service.GetClassrooms(ctx, query)
	return response, count, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError) {
	response, count, err := h.service.GetMyClassrooms(ctx, userID, query)
	return response, count, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError) {
	response, err := h.service.GetClassroom(ctx, userID, id)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, *httpx.HTTPError) {
	response, err := h.service.GetClassroomByCode(ctx, code)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, *httpx.HTTPError) {
	response, err := h.service.UpdateClassroom(ctx, teacherID, id, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError) {
	response, err := h.service.ArchiveClassroom(ctx, teacherID, id)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	response, err := h.service.RequestJoinClassroom(ctx, userID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	response, err := h.service.GetJoinRequests(ctx, teacherID, classroomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	response, err := h.service.ApproveMember(ctx, teacherID, classroomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError) {
	response, err := h.service.RejectMember(ctx, teacherID, classroomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) *httpx.HTTPError {
	err := h.service.RemoveMember(ctx, teacherID, classroomID, userID)
	return httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) CreateAssignments(ctx context.Context, teacherID string, req dto.CreateAssignmentsRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError) {
	response, err := h.service.CreateAssignments(ctx, teacherID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) AssignExistingExam(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.AssignExistingExamRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError) {
	response, err := h.service.AssignExistingExam(ctx, teacherID, classroomID, assignmentID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) UpdateAssignmentSchedule(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.UpdateAssignmentSchedule(ctx, teacherID, classroomID, assignmentID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int, *httpx.HTTPError) {
	response, count, err := h.service.GetAssignments(ctx, userID, classroomID, query)
	return response, count, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetAssignment(ctx context.Context, userID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.GetAssignment(ctx, userID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) SetAssignmentActive(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, active bool) (*dto.AssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.SetAssignmentActive(ctx, teacherID, classroomID, assignmentID, active)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetGradebook(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.GetGradebook(ctx, teacherID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetClassGradebook(ctx context.Context, teacherID string, classroomID uint, query dto.GradebookRangeQuery) (*dto.ClassGradebookResponse, *httpx.HTTPError) {
	response, err := h.service.GetClassGradebook(ctx, teacherID, classroomID, query)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetStudentProgress(ctx context.Context, userID string, classroomID uint, studentID string) (*dto.StudentProgressResponse, *httpx.HTTPError) {
	response, err := h.service.GetStudentProgress(ctx, userID, classroomID, studentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetAssignmentAnalytics(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentAnalyticsResponse, *httpx.HTTPError) {
	response, err := h.service.GetAssignmentAnalytics(ctx, teacherID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetEditableAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.EditableAssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.GetEditableAssignment(ctx, teacherID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) UpdateAssignmentContent(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.EditableAssignmentResponse, *httpx.HTTPError) {
	response, err := h.service.UpdateAssignmentContent(ctx, teacherID, classroomID, assignmentID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) ResetAssignmentAttempt(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, studentID string) *httpx.HTTPError {
	err := h.service.ResetAssignmentAttempt(ctx, teacherID, classroomID, assignmentID, studentID)
	return httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) ReconcileAssignmentSubmissions(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (int, *httpx.HTTPError) {
	response, err := h.service.ReconcileAssignmentSubmissions(ctx, teacherID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetAssignmentAudits(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) ([]*dto.ExamAuditResponse, *httpx.HTTPError) {
	response, err := h.service.GetAssignmentAudits(ctx, teacherID, classroomID, assignmentID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) CreateSchedule(ctx context.Context, teacherID string, classroomID uint, req dto.CreateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError) {
	response, err := h.service.CreateSchedule(ctx, teacherID, classroomID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetCalendar(ctx context.Context, userID string, classroomID uint, query dto.CalendarQuery) ([]*dto.CalendarResponse, *httpx.HTTPError) {
	response, err := h.service.GetCalendar(ctx, userID, classroomID, query)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) UpdateSchedule(ctx context.Context, teacherID string, scheduleID uint, req dto.UpdateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError) {
	response, err := h.service.UpdateSchedule(ctx, teacherID, scheduleID, req)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) DeleteSchedule(ctx context.Context, teacherID string, scheduleID uint) *httpx.HTTPError {
	err := h.service.DeleteSchedule(ctx, teacherID, scheduleID)
	return httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetRoom(ctx context.Context, userID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	response, err := h.service.GetRoom(ctx, userID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetRoomActivity(ctx context.Context, userID string, classroomID uint, roomID uint) (*dto.RoomActivityResponse, *httpx.HTTPError) {
	response, err := h.service.GetRoomActivity(ctx, userID, classroomID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) StartRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	response, err := h.service.StartRoom(ctx, teacherID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) JoinRoom(ctx context.Context, userID string, roomID uint) (*dto.JoinRoomResponse, *httpx.HTTPError) {
	response, err := h.service.JoinRoom(ctx, userID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) EndRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError) {
	response, err := h.service.EndRoom(ctx, teacherID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) GetRoomAttendance(ctx context.Context, userID string, roomID uint) ([]*dto.AttendanceSessionResponse, *httpx.HTTPError) {
	response, err := h.service.GetRoomAttendance(ctx, userID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RequestScreenShare(ctx context.Context, userID string, classroomID, roomID uint) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.RequestScreenShare(ctx, userID, classroomID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) SetHandRaised(ctx context.Context, userID string, classroomID, roomID uint, raised bool) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.SetHandRaised(ctx, userID, classroomID, roomID, raised)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) LowerHands(ctx context.Context, teacherID string, classroomID, roomID uint, userID *string) (*dto.LoweredHandsResponse, *httpx.HTTPError) {
	response, err := h.service.LowerHands(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) MuteParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.MuteParticipant(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) AllowParticipantMicrophone(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.AllowParticipantMicrophone(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) DisableParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.DisableParticipantCamera(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) AllowParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.AllowParticipantCamera(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) MuteAllParticipants(ctx context.Context, teacherID string, classroomID, roomID uint) (*dto.MutedParticipantsResponse, *httpx.HTTPError) {
	response, err := h.service.MuteAllParticipants(ctx, teacherID, classroomID, roomID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RemoveParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string, reason *string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.RemoveParticipant(ctx, teacherID, classroomID, roomID, userID, reason)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) ApproveScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.ApproveScreenShare(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) RejectScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError) {
	response, err := h.service.RejectScreenShare(ctx, teacherID, classroomID, roomID, userID)
	return response, httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) StartScreenShare(ctx context.Context, userID string, classroomID, roomID uint) *httpx.HTTPError {
	err := h.service.StartScreenShare(ctx, userID, classroomID, roomID)
	return httpx.FromError(err, "[ERROR] Classroom service failed")
}

func (h *HTTPService) StopScreenShare(ctx context.Context, actorID string, classroomID, roomID uint, userID string) *httpx.HTTPError {
	err := h.service.StopScreenShare(ctx, actorID, classroomID, roomID, userID)
	return httpx.FromError(err, "[ERROR] Classroom service failed")
}
