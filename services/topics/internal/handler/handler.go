package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	_ "ego/services/topics/internal/dto"
	"ego/services/topics/internal/service"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	GetTopics(w http.ResponseWriter, r *http.Request)
	GetSectionsByTopic(w http.ResponseWriter, r *http.Request)
	GetLessonsBySection(w http.ResponseWriter, r *http.Request)
	GetLessonByID(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(service service.Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("GET /topics", mw.Handle(h.GetTopics))
	mux.HandleFunc("GET /topics/{id}", mw.Handle(h.GetSectionsByTopic))
	mux.HandleFunc("GET /sections/{id}/lessons", mw.Handle(h.GetLessonsBySection))
	mux.HandleFunc("GET /lessons/{id}", mw.Handle(h.GetLessonByID))
}

// GetTopics godoc
// @Summary      Get topic list
// @Description  Get the list of all topics
// @Tags         Topics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.GetTopicsResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /topics [get]
func (h *handler) GetTopics(w http.ResponseWriter, r *http.Request) {
	topics, err := h.service.GetTopics(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, topics)
}

// GetSectionsByTopic godoc
// @Summary      Get sections by topic
// @Description  Get the list of all sections for a specific topic
// @Tags         Topics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Topic ID"
// @Success      200  {object}  dto.GetSectionsResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /topics/{id} [get]
func (h *handler) GetSectionsByTopic(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid topic ID")
		return
	}

	sections, err := h.service.GetSectionsByTopic(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, sections)
}

// GetLessonsBySection godoc
// @Summary      Get lessons by section
// @Description  Get the list of all lessons for a specific section
// @Tags         Topics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Section ID"
// @Success      200  {object}  dto.GetLessonsResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /sections/{id}/lessons [get]
func (h *handler) GetLessonsBySection(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid section ID")
		return
	}

	lessons, err := h.service.GetLessonsBySection(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, lessons)
}

// GetLessonByID godoc
// @Summary      Get lesson by ID
// @Description  Get lesson details including transcripts
// @Tags         Topics
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Lesson ID"
// @Success      200  {object}  dto.GetLessonByIDResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /lessons/{id} [get]
func (h *handler) GetLessonByID(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid lesson ID")
		return
	}

	resp, err := h.service.GetLessonByID(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
