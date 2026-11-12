package handler

import (
	"ego/platform/jwt"
	"ego/services/identity/internal/service"
	"net/http"
)

// Handler của từng domain được cài đặt ở <domain>.go và đăng ký trong RegisterRoutes.
type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)

	// Auth
	Login(w http.ResponseWriter, r *http.Request)
	Refresh(w http.ResponseWriter, r *http.Request)

	// User
	GetMe(w http.ResponseWriter, r *http.Request)
	UpdateMe(w http.ResponseWriter, r *http.Request)

	// User (admin)
	GetList(w http.ResponseWriter, r *http.Request)
	BlockUser(w http.ResponseWriter, r *http.Request)
	UnblockUser(w http.ResponseWriter, r *http.Request)

	// Stats (admin)
	GetAdminStats(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	// Auth (hỗ trợ cả prefix /auth/ và direct qua legacy gateway)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/refresh", h.Refresh)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("POST /refresh", h.Refresh)

	// User (hỗ trợ cả /users/me và /me khi gọi qua nginx location /users/)
	mux.HandleFunc("GET /users/me", mw.Handle(h.GetMe))
	mux.HandleFunc("PATCH /users/me", mw.Handle(h.UpdateMe))
	mux.HandleFunc("GET /me", mw.Handle(h.GetMe))
	mux.HandleFunc("PATCH /me", mw.Handle(h.UpdateMe))

	// User (admin)
	mux.HandleFunc("GET /users", mw.Handle(rm.RequireRole("admin")(h.GetList)))
	mux.HandleFunc("PUT /users/{id}/block", mw.Handle(rm.RequireRole("admin")(h.BlockUser)))
	mux.HandleFunc("DELETE /users/{id}/block", mw.Handle(rm.RequireRole("admin")(h.UnblockUser)))

	// Stats (admin)
	mux.HandleFunc("GET /admin/stats", mw.Handle(rm.RequireRole("admin")(h.GetAdminStats)))
}
