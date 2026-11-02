package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/dictionary/internal/dto"
	"ego/services/dictionary/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	LookupWord(w http.ResponseWriter, r *http.Request)
	SearchWords(w http.ResponseWriter, r *http.Request)
	GetThesaurus(w http.ResponseWriter, r *http.Request)
	GetFlashcards(w http.ResponseWriter, r *http.Request)
	AddFlashcard(w http.ResponseWriter, r *http.Request)
	DeleteFlashcard(w http.ResponseWriter, r *http.Request)
	ListTopics(w http.ResponseWriter, r *http.Request)
	GetTopicWords(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("GET /words/{word}", mw.Handle(h.LookupWord))
	mux.HandleFunc("GET /words", mw.Handle(h.SearchWords))
	mux.HandleFunc("GET /words/{word}/thesaurus", mw.Handle(h.GetThesaurus))
	mux.HandleFunc("GET /flashcards", mw.Handle(h.GetFlashcards))
	mux.HandleFunc("POST /flashcards", mw.Handle(h.AddFlashcard))
	mux.HandleFunc("DELETE /flashcards/{id}", mw.Handle(h.DeleteFlashcard))
	mux.HandleFunc("GET /topics", mw.Handle(h.ListTopics))
	mux.HandleFunc("GET /topics/{id}/words", mw.Handle(h.GetTopicWords))
}

// LookupWord godoc
// @Summary      Look up a word
// @Description  Get full dictionary entry for a word including phonetics, meanings, definitions, synonyms, antonyms
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        word  path      string  true  "Word to look up"
// @Success      200   {object}  dto.WordResponse
// @Failure      404   {object}  httpx.ErrorResponse
// @Failure      500   {object}  httpx.ErrorResponse
// @Router       /words/{word} [get]
func (h *handler) LookupWord(w http.ResponseWriter, r *http.Request) {
	word := r.PathValue("word")
	if word == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Word is required", nil))
		return
	}

	resp, err := h.service.LookupWord(r.Context(), word)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Word not found: "+word, nil))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to look up word", nil))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// SearchWords godoc
// @Summary      Search words
// @Description  Search dictionary words with prefix match. Supports pagination.
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        q         query     string  false  "Search query (prefix)"
// @Param        page      query     int     false  "Page number (default: 1)"
// @Param        pageSize  query     int     false  "Items per page (default: 10, max: 50)"
// @Success      200       {object}  dto.SearchResponse
// @Failure      500       {object}  httpx.ErrorResponse
// @Router       /words [get]
func (h *handler) SearchWords(w http.ResponseWriter, r *http.Request) {
	var q dto.SearchQuery
	if err := httpx.DecodeQuery(r, &q); err != nil {
		// ignore validation errors for optional query params
	}
	q.Normalize()

	resp, err := h.service.SearchWords(r.Context(), q.Q, q.Page, q.PageSize)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to search words", nil))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// GetThesaurus godoc
// @Summary      Get thesaurus for a word
// @Description  Returns all synonyms and antonyms for a word across all meanings
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        word  path      string  true  "Word"
// @Success      200   {object}  dto.ThesaurusResponse
// @Failure      404   {object}  httpx.ErrorResponse
// @Failure      500   {object}  httpx.ErrorResponse
// @Router       /words/{word}/thesaurus [get]
func (h *handler) GetThesaurus(w http.ResponseWriter, r *http.Request) {
	word := r.PathValue("word")
	if word == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Word is required", nil))
		return
	}

	resp, err := h.service.GetThesaurus(r.Context(), word)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Word not found: "+word, nil))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get thesaurus", nil))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// GetFlashcards godoc
// @Summary      Get user flashcards
// @Description  Get the authenticated user's saved flashcards
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int  false  "Page number (default: 1)"
// @Param        pageSize  query     int  false  "Items per page (default: 10, max: 100)"
// @Success      200       {object}  dto.GetFlashcardsResponse
// @Failure      401       {object}  httpx.ErrorResponse
// @Failure      500       {object}  httpx.ErrorResponse
// @Router       /flashcards [get]
func (h *handler) GetFlashcards(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	var q dto.GetFlashcardsQuery
	if err := httpx.DecodeQuery(r, &q); err != nil {
		// ignore
	}
	q.Normalize()

	resp, err := h.service.GetFlashcards(r.Context(), userID, q.Page, q.PageSize)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get flashcards", nil))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// AddFlashcard godoc
// @Summary      Add a flashcard
// @Description  Save a word as a flashcard for the authenticated user
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      dto.AddFlashcardRequest  true  "Flashcard request"
// @Success      201      {object}  dto.FlashcardResponse
// @Failure      400      {object}  httpx.ErrorResponse
// @Failure      401      {object}  httpx.ErrorResponse
// @Failure      404      {object}  httpx.ErrorResponse
// @Failure      500      {object}  httpx.ErrorResponse
// @Router       /flashcards [post]
func (h *handler) AddFlashcard(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	var body dto.AddFlashcardRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body: "+err.Error(), nil))
		return
	}

	resp, err := h.service.AddFlashcard(r.Context(), userID, body.WordID, body.Note)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Word not found", nil))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to add flashcard", nil))
		return
	}

	httpx.JSON(w, http.StatusCreated, resp)
}

// DeleteFlashcard godoc
// @Summary      Delete a flashcard
// @Description  Remove a flashcard from the authenticated user's collection
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path      int  true  "Flashcard ID"
// @Success      204
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /flashcards/{id} [delete]
func (h *handler) DeleteFlashcard(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid flashcard ID", nil))
		return
	}

	if err := h.service.DeleteFlashcard(r.Context(), userID, uint(id)); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Flashcard not found", nil))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to delete flashcard", nil))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTopics godoc
// @Summary      List all vocabulary topics
// @Description  Get a list of all vocabulary categories and topics
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200   {array}   dto.TopicResponse
// @Failure      500   {object}  httpx.ErrorResponse
// @Router       /topics [get]
func (h *handler) ListTopics(w http.ResponseWriter, r *http.Request) {
	resp, err := h.service.ListTopics(r.Context())
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to list topics", nil))
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetTopicWords godoc
// @Summary      Get words in a topic
// @Description  Get paginated list of words for a specific topic
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      string  true   "Topic ID"
// @Param        page      query     int     false  "Page number (default 1)"
// @Param        pageSize  query     int     false  "Page size (default 20, max 100)"
// @Success      200       {object}  dto.GetTopicWordsResponse
// @Failure      400       {object}  httpx.ErrorResponse
// @Failure      404       {object}  httpx.ErrorResponse
// @Failure      500       {object}  httpx.ErrorResponse
// @Router       /topics/{id}/words [get]
func (h *handler) GetTopicWords(w http.ResponseWriter, r *http.Request) {
	topicID := r.PathValue("id")
	if topicID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Topic ID is required", nil))
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	q := dto.GetTopicWordsQuery{
		Page:     page,
		PageSize: pageSize,
	}
	q.Normalize()

	resp, err := h.service.GetTopicWords(r.Context(), topicID, q.Page, q.PageSize)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Topic not found: "+topicID, nil))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get topic words", nil))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}