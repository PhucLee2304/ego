package handler

import (
	"errors"
	"net/http"
	"strconv"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/vocabgram/internal/dto"
	"ego/services/vocabgram/internal/service"
)

// GetCurriculum godoc
// @Summary      Get grammar curriculum
// @Description  Returns all grammar units with per-user progress. Shows completed/total status for each unit.
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.CurriculumResponse
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /curriculum [get]
func (h *handler) GetCurriculum(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	resp, err := h.service.GetCurriculum(r.Context(), userID)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get curriculum", err))
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
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /units/{id} [get]
func (h *handler) GetUnit(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid unit ID", err))
		return
	}

	resp, err := h.service.GetUnit(r.Context(), userID, uint(id))
	if err != nil {
		if errors.Is(err, service.ErrGrammarNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Unit not found", err))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get unit", err))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// SubmitProgress godoc
// @Summary      Submit practice completion for a unit
// @Description  Mark a grammar unit as completed (or in-progress) after finishing practice questions.
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                        true  "Unit ID"
// @Param        request  body      dto.SubmitProgressRequest  true  "Progress payload"
// @Success      200      {object}  dto.ProgressResponse
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /units/{id}/progress [post]
func (h *handler) SubmitProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok || userID == "" {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid unit ID", err))
		return
	}

	var req dto.SubmitProgressRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, err := h.service.SubmitProgress(r.Context(), userID, uint(id), req.Completed)
	if err != nil {
		if errors.Is(err, service.ErrGrammarNotFound) {
			httpx.Error(w, httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Unit not found", err))
			return
		}
		httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to submit progress", err))
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
