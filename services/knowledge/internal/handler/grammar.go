package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/knowledge/internal/dto"
)

// GetUnits godoc
// @Summary      Get grammar units
// @Description  All grammar units ordered by unit number, with the current user's progress
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.GetUnitsResponse
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /units [get]
func (h *handler) GetUnits(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	units, httpErr := h.service.GetUnits(r.Context(), userID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, units)
}

// GetUnitByID godoc
// @Summary      Get grammar unit detail
// @Description  Theory, examples and practice questions of a unit, with the current user's progress
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Unit ID"
// @Success      200  {object}  dto.Unit
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /units/{id} [get]
func (h *handler) GetUnitByID(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid unit ID", err))
		return
	}

	unit, httpErr := h.service.GetUnitByID(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, unit)
}

// UpdateProgress godoc
// @Summary      Update unit progress
// @Description  Mark the current user's practice of a unit as completed or not
// @Tags         Grammar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                     true  "Unit ID"
// @Param        request  body      dto.UpdateProgressBody  true  "Progress"
// @Success      200      {object}  dto.Progress
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /units/{id}/progress [put]
func (h *handler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid unit ID", err))
		return
	}

	var body dto.UpdateProgressBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	progress, httpErr := h.service.UpdateProgress(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, progress)
}
