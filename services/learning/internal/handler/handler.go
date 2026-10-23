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
}

// RegisterSockets mount 2 hub WebSocket riêng, giữ nguyên đường dẫn của exams và classrooms.
func RegisterSockets(mux *http.ServeMux, examsHub, classroomsHub http.Handler) {
	mux.Handle("/ws/exams", examsHub)
	mux.Handle("/ws/classrooms", classroomsHub)
}
