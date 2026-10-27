package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// GetAttemptNavigator godoc
// @Summary      Get attempt navigator
// @Description  Get pages and question states (answered/locked) of an attempt for the question palette; works for every flow
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Attempt ID"
// @Success      200  {object}  dto.AttemptNavigatorResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/navigator [get]
func (h *handler) GetAttemptNavigator(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var resp *dto.AttemptNavigatorResponse
	resp, httpErr := h.service.GetAttemptNavigator(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// GetAttemptPage godoc
// @Summary      Get attempt page
// @Description  Get content of one page of an attempt. In TOEIC_STRICT flow, passed/future listening pages and reading pages during listening return 409 PAGE_LOCKED
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id     path      int  true  "Attempt ID"
// @Param        index  path      int  true  "Page index (0-based)"
// @Success      200  {object}  dto.AttemptPageResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/pages/{index} [get]
func (h *handler) GetAttemptPage(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}
	index, err := httpx.ParsePathID(r, "index")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid page index", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var resp *dto.AttemptPageResponse
	resp, httpErr := h.service.GetAttemptPage(r.Context(), userID, id, int(index))
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
