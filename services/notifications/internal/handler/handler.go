package handler

import (
	"net/http"

	"ego/platform/jwt"
	"ego/services/notifications/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)

	// Notification
	GetNotifications(w http.ResponseWriter, r *http.Request)
	MarkRead(w http.ResponseWriter, r *http.Request)
	DeleteNotifications(w http.ResponseWriter, r *http.Request)

	// Device token
	RegisterDeviceToken(w http.ResponseWriter, r *http.Request)
	DeleteDeviceToken(w http.ResponseWriter, r *http.Request)

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
	// Notification
	mux.HandleFunc("GET /notifications", mw.Handle(h.GetNotifications))
	mux.HandleFunc("PUT /notifications/read", mw.Handle(h.MarkRead))
	mux.HandleFunc("DELETE /notifications", mw.Handle(h.DeleteNotifications))

	// Device token
	mux.HandleFunc("POST /notifications/device-tokens", mw.Handle(h.RegisterDeviceToken))
	mux.HandleFunc("DELETE /notifications/device-tokens", mw.Handle(h.DeleteDeviceToken))

	// Stats (admin)
	mux.HandleFunc("GET /admin/stats", mw.Handle(rm.RequireRole("admin")(h.GetAdminStats)))
}
