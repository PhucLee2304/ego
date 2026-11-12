package handler

import (
	"ego/platform/jwt"
	"ego/services/knowledge/internal/service"
	"net/http"
)

// Handler gom HTTP handler của mọi domain trong knowledge (topic, dictionary, grammar).
// Handler của từng domain được cài đặt ở <domain>.go và đăng ký trong RegisterRoutes.
type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)

	// Topic
	GetTopics(w http.ResponseWriter, r *http.Request)
	GetSectionsByTopic(w http.ResponseWriter, r *http.Request)
	GetLessonsBySection(w http.ResponseWriter, r *http.Request)
	GetLessonByID(w http.ResponseWriter, r *http.Request)

	// Word
	GetWords(w http.ResponseWriter, r *http.Request)
	GetWordByID(w http.ResponseWriter, r *http.Request)

	// Flashcard Deck
	CreateDeck(w http.ResponseWriter, r *http.Request)
	GetDecks(w http.ResponseWriter, r *http.Request)
	GetDeckByID(w http.ResponseWriter, r *http.Request)
	UpdateDeck(w http.ResponseWriter, r *http.Request)
	DeleteDeck(w http.ResponseWriter, r *http.Request)

	// Flashcard Card & Study
	SaveWordToDecks(w http.ResponseWriter, r *http.Request)
	GetWordSaveStatus(w http.ResponseWriter, r *http.Request)
	GetCardsByDeck(w http.ResponseWriter, r *http.Request)
	UpdateFlashcardNote(w http.ResponseWriter, r *http.Request)
	DeleteFlashcard(w http.ResponseWriter, r *http.Request)
	GetStudyCards(w http.ResponseWriter, r *http.Request)
	GetAllDueCards(w http.ResponseWriter, r *http.Request)
	ReviewFlashcard(w http.ResponseWriter, r *http.Request)
	GetFlashcardUserStats(w http.ResponseWriter, r *http.Request)

	// Theme
	GetThemes(w http.ResponseWriter, r *http.Request)
	GetWordsByTheme(w http.ResponseWriter, r *http.Request)

	// Grammar
	GetUnits(w http.ResponseWriter, r *http.Request)
	GetUnitByID(w http.ResponseWriter, r *http.Request)
	UpdateProgress(w http.ResponseWriter, r *http.Request)

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
	// Topic
	mux.HandleFunc("GET /topics", mw.Handle(h.GetTopics))
	mux.HandleFunc("GET /topics/{id}", mw.Handle(h.GetSectionsByTopic))
	mux.HandleFunc("GET /sections/{id}/lessons", mw.Handle(h.GetLessonsBySection))
	mux.HandleFunc("GET /lessons/{id}", mw.Handle(h.GetLessonByID))

	// Word
	mux.HandleFunc("GET /words", mw.Handle(h.GetWords))
	mux.HandleFunc("GET /words/{id}", mw.Handle(h.GetWordByID))

	// Flashcard Decks
	mux.HandleFunc("POST /flashcard-decks", mw.Handle(h.CreateDeck))
	mux.HandleFunc("GET /flashcard-decks", mw.Handle(h.GetDecks))
	mux.HandleFunc("GET /flashcard-decks/{id}", mw.Handle(h.GetDeckByID))
	mux.HandleFunc("PUT /flashcard-decks/{id}", mw.Handle(h.UpdateDeck))
	mux.HandleFunc("DELETE /flashcard-decks/{id}", mw.Handle(h.DeleteDeck))

	// Flashcard Cards & Study
	mux.HandleFunc("POST /flashcards/save-word", mw.Handle(h.SaveWordToDecks))
	mux.HandleFunc("GET /flashcards/status", mw.Handle(h.GetWordSaveStatus))
	mux.HandleFunc("GET /flashcard-decks/{id}/cards", mw.Handle(h.GetCardsByDeck))
	mux.HandleFunc("PUT /flashcards/{id}/note", mw.Handle(h.UpdateFlashcardNote))
	mux.HandleFunc("DELETE /flashcards/{id}", mw.Handle(h.DeleteFlashcard))
	mux.HandleFunc("GET /flashcard-decks/{id}/study", mw.Handle(h.GetStudyCards))
	mux.HandleFunc("GET /flashcards/study/due-all", mw.Handle(h.GetAllDueCards))
	mux.HandleFunc("POST /flashcards/{id}/review", mw.Handle(h.ReviewFlashcard))
	mux.HandleFunc("GET /flashcards/stats", mw.Handle(h.GetFlashcardUserStats))

	// Theme
	mux.HandleFunc("GET /themes", mw.Handle(h.GetThemes))
	mux.HandleFunc("GET /themes/{id}", mw.Handle(h.GetWordsByTheme))

	// Grammar
	mux.HandleFunc("GET /units", mw.Handle(h.GetUnits))
	mux.HandleFunc("GET /units/{id}", mw.Handle(h.GetUnitByID))
	mux.HandleFunc("PUT /units/{id}/progress", mw.Handle(h.UpdateProgress))

	// Stats (admin)
	mux.HandleFunc("GET /admin/stats", mw.Handle(rm.RequireRole("admin")(h.GetAdminStats)))
}
