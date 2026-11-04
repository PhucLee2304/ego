package handler

import (
	"ego/platform/jwt"
	"ego/services/knowledge/internal/service"
	"net/http"
)

// Handler gom HTTP handler của mọi domain trong knowledge (topic, dictionary, grammar).
// Handler của từng domain được cài đặt ở <domain>.go và đăng ký trong RegisterRoutes.
type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
}
