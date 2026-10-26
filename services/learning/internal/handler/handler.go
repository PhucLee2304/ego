package handler

import (
	"net/http"

	"ego/platform/jwt"
	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)
	RegisterSocketHandlers(examsHub *platformsocket.Server)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	// Exam
	mux.HandleFunc("GET /exams", mw.Handle(h.GetList))
	mux.HandleFunc("GET /exams/{id}", mw.Handle(h.GetByID))

	// Attempt
	mux.HandleFunc("POST /exams/{id}/attempts", mw.Handle(h.CreateAttempt))
	mux.HandleFunc("GET /attempts", mw.Handle(h.GetAttempts))
	mux.HandleFunc("GET /attempts/{id}/questions", mw.Handle(h.GetAttemptQuestions))
	mux.HandleFunc("GET /attempts/{id}/navigator", mw.Handle(h.GetAttemptNavigator))
	mux.HandleFunc("GET /attempts/{id}/pages/{index}", mw.Handle(h.GetAttemptPage))
	mux.HandleFunc("POST /attempts/{id}/submit", mw.Handle(h.SubmitAttempt))
	mux.HandleFunc("POST /attempts/{id}/cancel", mw.Handle(h.CancelAttempt))
	mux.HandleFunc("GET /attempts/{id}/history", mw.Handle(h.GetAttemptHistory))

	// Classroom
	mux.HandleFunc("POST /classrooms", mw.Handle(h.CreateClassroom))
	mux.HandleFunc("GET /classrooms", mw.Handle(rm.RequireRole("admin")(h.GetClassrooms)))
	mux.HandleFunc("GET /classrooms/me", mw.Handle(h.GetMyClassrooms))
	mux.HandleFunc("GET /classrooms/code", mw.Handle(h.GetClassroomByCode))
	mux.HandleFunc("GET /classrooms/{id}", mw.Handle(h.GetClassroom))
	mux.HandleFunc("PATCH /classrooms/{id}", mw.Handle(h.UpdateClassroom))
	mux.HandleFunc("POST /classrooms/{id}/archive", mw.Handle(h.ArchiveClassroom))

	// Member
	mux.HandleFunc("POST /classrooms/join", mw.Handle(h.RequestJoinClassroom))
	mux.HandleFunc("GET /classrooms/{id}/members/join-requests", mw.Handle(h.GetJoinRequests))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/approve", mw.Handle(h.ApproveMember))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/reject", mw.Handle(h.RejectMember))
	mux.HandleFunc("DELETE /classrooms/{id}/members/{userId}", mw.Handle(h.RemoveMember))

	// Assignment
	mux.HandleFunc("POST /classrooms/assignments", mw.Handle(h.CreateAssignments))
	mux.HandleFunc("GET /classrooms/{id}/assignments", mw.Handle(h.GetAssignments))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}", mw.Handle(h.GetAssignment))
	mux.HandleFunc("POST /classrooms/{id}/assignments/{assignmentId}/classrooms", mw.Handle(h.AssignExistingExam))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}/schedule", mw.Handle(h.UpdateAssignmentSchedule))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}/active", mw.Handle(h.SetAssignmentActive))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/edit", mw.Handle(h.GetEditableAssignment))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}", mw.Handle(h.UpdateAssignmentContent))
	mux.HandleFunc("DELETE /classrooms/{id}/assignments/{assignmentId}/students/{studentId}/attempt", mw.Handle(h.ResetAssignmentAttempt))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/audits", mw.Handle(h.GetAssignmentAudits))

	// Gradebook
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/gradebook", mw.Handle(h.GetGradebook))
	mux.HandleFunc("GET /classrooms/{id}/gradebook", mw.Handle(h.GetClassGradebook))
	mux.HandleFunc("GET /classrooms/{id}/students/{userId}/progress", mw.Handle(h.GetStudentProgress))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/analytics", mw.Handle(h.GetAssignmentAnalytics))
	mux.HandleFunc("POST /classrooms/{id}/assignments/{assignmentId}/gradebook/reconcile", mw.Handle(h.ReconcileAssignmentSubmissions))

	// Schedule
	mux.HandleFunc("POST /classrooms/{id}/schedules", mw.Handle(h.CreateSchedule))
	mux.HandleFunc("GET /classrooms/{id}/calendar", mw.Handle(h.GetCalendar))
	mux.HandleFunc("PATCH /classrooms/{id}/schedules/{scheduleId}", mw.Handle(h.UpdateSchedule))
	mux.HandleFunc("DELETE /classrooms/{id}/schedules/{scheduleId}", mw.Handle(h.DeleteSchedule))

	// Room
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}", mw.Handle(h.GetRoom))
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}/activity", mw.Handle(h.GetRoomActivity))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/start", mw.Handle(h.StartRoom))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/join", mw.Handle(h.JoinRoom))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/end", mw.Handle(h.EndRoom))
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}/attendance", mw.Handle(h.GetRoomAttendance))

	// Screen share
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/request", mw.Handle(h.RequestScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/start", mw.Handle(h.StartScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/approve", mw.Handle(h.ApproveScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/reject", mw.Handle(h.RejectScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/stop", mw.Handle(h.StopScreenShare))

	// Hands
	mux.HandleFunc("PATCH /classrooms/{id}/rooms/{roomId}/hand", mw.Handle(h.SetHandRaised))
	mux.HandleFunc("DELETE /classrooms/{id}/rooms/{roomId}/hands", mw.Handle(h.LowerHands))
	mux.HandleFunc("DELETE /classrooms/{id}/rooms/{roomId}/hands/{userId}", mw.Handle(h.LowerHands))

	// Moderation
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/mute-all", mw.Handle(h.MuteAllParticipants))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/mute", mw.Handle(h.MuteParticipant))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-microphone", mw.Handle(h.AllowParticipantMicrophone))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/disable-camera", mw.Handle(h.DisableParticipantCamera))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-camera", mw.Handle(h.AllowParticipantCamera))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/remove", mw.Handle(h.RemoveParticipant))
}

// RegisterSockets mount 2 hub WebSocket riêng, giữ nguyên đường dẫn của exams và classrooms.
func RegisterSockets(mux *http.ServeMux, examsHub, classroomsHub http.Handler) {
	mux.Handle("/ws/exams", examsHub)
	mux.Handle("/ws/classrooms", classroomsHub)
}
