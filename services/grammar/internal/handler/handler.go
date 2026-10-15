package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/grammar/internal/dto"
	"ego/services/grammar/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
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
	// Curriculum dashboard — requires auth to load personalised progress
	mux.HandleFunc("GET /curriculum", mw.Handle(h.GetCurriculum))

	// Lesson screen — requires auth for personalised progress
	mux.HandleFunc("GET /units/{id}", mw.Handle(h.GetUnit))

	// Submit progress after practice session
	mux.HandleFunc("POST /units/{id}/progress", mw.Handle(h.SubmitProgress))
}

// GetCurriculum godoc
// @Summary      Get grammar curriculum
// @Description  Returns all 11 grammar units with per-user progress (gamification). Shows completed/total status for each unit.
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.CurriculumResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /curriculum [get]
func (h *handler) GetCurriculum(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated")
		return
	}

	resp, err := h.service.GetCurriculum(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "[ERROR] Failed to get curriculum")
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// GetUnit godoc
// @Summary      Get a grammar unit (Lesson Screen)
// @Description  Returns the full lesson data for a unit: theory, examples, and practice questions. Includes per-user progress status.
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Unit ID"
// @Success      200  {object}  dto.UnitDetailResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /units/{id} [get]
func (h *handler) GetUnit(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid unit ID")
		return
	}

	resp, err := h.service.GetUnit(r.Context(), userID, uint(id))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "[NOT_FOUND] Unit not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "[ERROR] Failed to get unit")
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// SubmitProgress godoc
// @Summary      Submit practice progress
// @Description  Marks a grammar unit as completed (or not) for the authenticated user. Called after the user finishes the practice engine.
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                          true  "Unit ID"
// @Param        request  body      dto.SubmitProgressRequest    true  "Progress payload"
// @Success      200      {object}  dto.ProgressResponse
// @Failure      400      {object}  httpx.ErrorResponse
// @Failure      401      {object}  httpx.ErrorResponse
// @Failure      404      {object}  httpx.ErrorResponse
// @Failure      500      {object}  httpx.ErrorResponse
// @Router       /units/{id}/progress [post]
func (h *handler) SubmitProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated")
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid unit ID")
		return
	}

	var body dto.SubmitProgressRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body: "+err.Error())
		return
	}

	resp, err := h.service.SubmitProgress(r.Context(), userID, uint(id), body.Completed)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			httpx.Error(w, http.StatusNotFound, "[NOT_FOUND] Unit not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "[ERROR] Failed to submit progress")
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
