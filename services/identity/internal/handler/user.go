package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/identity/internal/dto"
	"net/http"
)

// GetMe godoc
// @Summary      Get current user info
// @Description  Get the current authenticated user's information
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.User
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /me [get]
func (h *handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	user, httpErr := h.service.GetMe(r.Context(), userID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, user)
}

// UpdateMe godoc
// @Summary      Update current user info
// @Description  Update the current authenticated user's information
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.UpdateUserBody true "Update User Request"
// @Success      200  {object}  dto.User
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /me [patch]
func (h *handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var body dto.UpdateUserBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	if body.Name == nil && body.Avatar == nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] No fields to update", nil))
		return
	}

	user, httpErr := h.service.UpdateMe(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, user)
}

// GetList godoc
// @Summary      Get user list
// @Description  Get the list of all users
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page     query    int  false "Page number"
// @Param        pageSize query    int  false "Page size"
// @Success      200  {object}  httpx.PaginatedResponse[dto.User]
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /users [get]
func (h *handler) GetList(w http.ResponseWriter, r *http.Request) {
	var query httpx.PaginationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	users, pageCounts, httpErr := h.service.GetList(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(users, query, pageCounts))
}
