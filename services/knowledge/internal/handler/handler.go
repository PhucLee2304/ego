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

	// Word
	GetWords(w http.ResponseWriter, r *http.Request)
	GetWordByID(w http.ResponseWriter, r *http.Request)

	// Flashcard
	GetFlashcards(w http.ResponseWriter, r *http.Request)
	CreateFlashcard(w http.ResponseWriter, r *http.Request)
	DeleteFlashcard(w http.ResponseWriter, r *http.Request)

	// Theme
	GetThemes(w http.ResponseWriter, r *http.Request)
	GetWordsByTheme(w http.ResponseWriter, r *http.Request)

	// Grammar
	GetUnits(w http.ResponseWriter, r *http.Request)
	GetUnitByID(w http.ResponseWriter, r *http.Request)
	UpdateProgress(w http.ResponseWriter, r *http.Request)
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

	// Word
	mux.HandleFunc("GET /words", mw.Handle(h.GetWords))
	mux.HandleFunc("GET /words/{id}", mw.Handle(h.GetWordByID))

	// Flashcard
	mux.HandleFunc("GET /flashcards", mw.Handle(h.GetFlashcards))
	mux.HandleFunc("POST /flashcards", mw.Handle(h.CreateFlashcard))
	mux.HandleFunc("DELETE /flashcards/{id}", mw.Handle(h.DeleteFlashcard))

	// Theme
	mux.HandleFunc("GET /themes", mw.Handle(h.GetThemes))
	mux.HandleFunc("GET /themes/{id}", mw.Handle(h.GetWordsByTheme))

	// Grammar
	mux.HandleFunc("GET /units", mw.Handle(h.GetUnits))
	mux.HandleFunc("GET /units/{id}", mw.Handle(h.GetUnitByID))
	mux.HandleFunc("PUT /units/{id}/progress", mw.Handle(h.UpdateProgress))
}
