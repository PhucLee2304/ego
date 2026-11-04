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

	// Topic
	GetTopics(w http.ResponseWriter, r *http.Request)
	GetSectionsByTopic(w http.ResponseWriter, r *http.Request)
	GetLessonsBySection(w http.ResponseWriter, r *http.Request)
	GetLessonByID(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	// Topic
	mux.HandleFunc("GET /topics", mw.Handle(h.GetTopics))
	mux.HandleFunc("GET /topics/{id}", mw.Handle(h.GetSectionsByTopic))
	mux.HandleFunc("GET /sections/{id}/lessons", mw.Handle(h.GetLessonsBySection))
	mux.HandleFunc("GET /lessons/{id}", mw.Handle(h.GetLessonByID))
}
