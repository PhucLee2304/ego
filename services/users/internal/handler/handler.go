package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/users/internal/dto"
	"ego/services/users/internal/service"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, roleMw *jwt.RoleMiddleware)
	GetMe(w http.ResponseWriter, r *http.Request)
	UpdateMe(w http.ResponseWriter, r *http.Request)
	GetList(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(service service.Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	mux.HandleFunc("GET /me", mw.Handle(h.GetMe))
	mux.HandleFunc("PATCH /me", mw.Handle(h.UpdateMe))
	mux.HandleFunc("GET /users", mw.Handle(rm.RequireRole("admin")(h.GetList)))
}

// GetMe godoc
// @Summary      Get current user info
// @Description  Get the current authenticated user's information
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.User
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /me [get]
func (h *handler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	user, err := h.service.GetMe(r.Context(), userID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /me [patch]
func (h *handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var body dto.UpdateUserBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}

	if body.Name == nil && body.Avatar == nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] No fields to update")
		return
	}

	user, err := h.service.UpdateMe(r.Context(), userID, body)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /users [get]
func (h *handler) GetList(w http.ResponseWriter, r *http.Request) {
	var query httpx.PaginationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()

	users, pageCounts, err := h.service.GetList(r.Context(), query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.PaginatedResponse[*dto.User]{
		Data:       users,
		Page:       int(query.Page),
		PageCounts: pageCounts,
	})
}
