package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// CreateClassroom godoc
// @Summary      Create classroom
// @Description  Create a new classroom for the current authenticated user.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateClassroomRequest true "Create classroom request"
// @Success      201 {object} dto.CreatedClassroomResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms [post]
func (h *handler) CreateClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var body dto.CreateClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.CreateClassroom(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// GetClassrooms godoc
// @Summary      Get classrooms
// @Description  Get all classrooms for admin management.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Param        query query string false "Search by classroom name, description or teacher name"
// @Param        active query bool false "Filter by active state"
// @Success      200 {object} httpx.PaginatedResponse[dto.ClassroomResponse]
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms [get]
func (h *handler) GetClassrooms(w http.ResponseWriter, r *http.Request) {
	var query dto.ClassroomsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	resp, total, httpErr := h.service.GetClassrooms(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, total))
}

// GetMyClassrooms godoc
// @Summary      Get my classrooms
// @Description  Get classrooms where the current user is the teacher or an approved student.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Param        query query string false "Search by classroom name, description or teacher name"
// @Param        active query bool false "Filter by active state"
// @Success      200 {object} httpx.PaginatedResponse[dto.ClassroomResponse]
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/me [get]
func (h *handler) GetMyClassrooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var query dto.ClassroomsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	resp, total, httpErr := h.service.GetMyClassrooms(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, total))
}

// GetClassroom godoc
// @Summary      Get classroom by ID
// @Description  Get classroom detail when the current user is the teacher or an approved student.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Success      200 {object} dto.ClassroomResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id} [get]
func (h *handler) GetClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}

	resp, httpErr := h.service.GetClassroom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetClassroomByCode godoc
// @Summary      Get classroom by code
// @Description  Preview an active classroom by code before joining.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code query string true "Classroom code"
// @Success      200 {object} dto.ClassroomByCodeResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/code [get]
func (h *handler) GetClassroomByCode(w http.ResponseWriter, r *http.Request) {
	var query dto.GetClassroomByCodeQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}

	resp, httpErr := h.service.GetClassroomByCode(r.Context(), query.Code)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// UpdateClassroom godoc
// @Summary      Update classroom
// @Description  Update classroom information. Only the owner teacher can update.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        request body dto.UpdateClassroomRequest true "Update classroom request"
// @Success      200 {object} dto.ClassroomResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id} [patch]
func (h *handler) UpdateClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}

	var body dto.UpdateClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	resp, httpErr := h.service.UpdateClassroom(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ArchiveClassroom godoc
// @Summary      Archive classroom
// @Description  Archive a classroom by setting active to false. Only the owner teacher can archive.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Success      200 {object} dto.ClassroomResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/archive [post]
func (h *handler) ArchiveClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}

	resp, httpErr := h.service.ArchiveClassroom(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
