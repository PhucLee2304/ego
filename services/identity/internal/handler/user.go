package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/identity/internal/dto"
	"net/http"
	"strconv"
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
// @Description  Admin: list users, searchable by name or email and filterable by role and blocked state.
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        query    query    string false "Search by name or email"
// @Param        role     query    string false "CLIENT or ADMIN"
// @Param        blocked  query    bool   false "true: blocked only, false: active only"
// @Param        page     query    int    false "Page number"
// @Param        pageSize query    int    false "Page size"
// @Success      200  {object}  httpx.PaginatedResponse[dto.User]
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /users [get]
func (h *handler) GetList(w http.ResponseWriter, r *http.Request) {
	var query dto.UsersQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	users, total, httpErr := h.service.GetList(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(users, query.PaginationQuery, total))
}

// GetSummary godoc
// @Summary      Get user summary
// @Description  Admin: total users, admins and blocked accounts.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.UsersSummary
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /users/summary [get]
func (h *handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	summary, httpErr := h.service.GetSummary(r.Context())
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, summary)
}

// BlockUser godoc
// @Summary      Block a user
// @Description  Admin: block a CLIENT account. It can no longer log in or refresh its session.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  dto.User
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Router       /users/{id}/block [post]
func (h *handler) BlockUser(w http.ResponseWriter, r *http.Request) {
	h.setBlocked(w, r, true)
}

// UnblockUser godoc
// @Summary      Unblock a user
// @Description  Admin: unblock a CLIENT account.
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  dto.User
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Router       /users/{id}/block [delete]
func (h *handler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	h.setBlocked(w, r, false)
}

func (h *handler) setBlocked(w http.ResponseWriter, r *http.Request, blocked bool) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid user ID", err))
		return
	}
	user, httpErr := h.service.SetBlocked(r.Context(), strconv.FormatUint(uint64(id), 10), blocked)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, user)
}
