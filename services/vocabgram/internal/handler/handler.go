package handler

import (
	"net/http"

	"ego/platform/jwt"
	"ego/services/vocabgram/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)

	// Dictionary
	LookupWord(w http.ResponseWriter, r *http.Request)
	SearchWords(w http.ResponseWriter, r *http.Request)
	GetThesaurus(w http.ResponseWriter, r *http.Request)
	GetFlashcards(w http.ResponseWriter, r *http.Request)
	AddFlashcard(w http.ResponseWriter, r *http.Request)
	DeleteFlashcard(w http.ResponseWriter, r *http.Request)
	ListTopics(w http.ResponseWriter, r *http.Request)
	GetTopicWords(w http.ResponseWriter, r *http.Request)

	// Grammar
	GetCurriculum(w http.ResponseWriter, r *http.Request)
	GetUnit(w http.ResponseWriter, r *http.Request)
	SubmitProgress(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	// Dictionary routes
	mux.HandleFunc("GET /words/{word}", mw.Handle(h.LookupWord))
	mux.HandleFunc("GET /words", mw.Handle(h.SearchWords))
	mux.HandleFunc("GET /words/{word}/thesaurus", mw.Handle(h.GetThesaurus))
	mux.HandleFunc("GET /flashcards", mw.Handle(h.GetFlashcards))
	mux.HandleFunc("POST /flashcards", mw.Handle(h.AddFlashcard))
	mux.HandleFunc("DELETE /flashcards/{id}", mw.Handle(h.DeleteFlashcard))
	mux.HandleFunc("GET /topics", mw.Handle(h.ListTopics))
	mux.HandleFunc("GET /topics/{id}/words", mw.Handle(h.GetTopicWords))

	// Grammar routes
	mux.HandleFunc("GET /curriculum", mw.Handle(h.GetCurriculum))
	mux.HandleFunc("GET /units/{id}", mw.Handle(h.GetUnit))
	mux.HandleFunc("POST /units/{id}/progress", mw.Handle(h.SubmitProgress))
}

