package handler

import (
	"net/http"

	"ego/platform/jwt"
	"ego/services/learning/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
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
}

// RegisterSockets mount 2 hub WebSocket riêng, giữ nguyên đường dẫn của exams và classrooms.
func RegisterSockets(mux *http.ServeMux, examsHub, classroomsHub http.Handler) {
	mux.Handle("/ws/exams", examsHub)
	mux.Handle("/ws/classrooms", classroomsHub)
}
