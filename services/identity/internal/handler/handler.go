package handler

import (
	"ego/platform/jwt"
	"ego/services/identity/internal/service"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)
	Login(w http.ResponseWriter, r *http.Request)
	Refresh(w http.ResponseWriter, r *http.Request)
	GetMe(w http.ResponseWriter, r *http.Request)
	UpdateMe(w http.ResponseWriter, r *http.Request)
	GetList(w http.ResponseWriter, r *http.Request)
	GetSummary(w http.ResponseWriter, r *http.Request)
	BlockUser(w http.ResponseWriter, r *http.Request)
	UnblockUser(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	mux.HandleFunc("POST /refresh", h.Refresh)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("GET /me", mw.Handle(h.GetMe))
	mux.HandleFunc("PATCH /me", mw.Handle(h.UpdateMe))
	mux.HandleFunc("GET /users", mw.Handle(rm.RequireRole("admin")(h.GetList)))
	mux.HandleFunc("GET /users/summary", mw.Handle(rm.RequireRole("admin")(h.GetSummary)))
	mux.HandleFunc("POST /users/{id}/block", mw.Handle(rm.RequireRole("admin")(h.BlockUser)))
	mux.HandleFunc("DELETE /users/{id}/block", mw.Handle(rm.RequireRole("admin")(h.UnblockUser)))
}
