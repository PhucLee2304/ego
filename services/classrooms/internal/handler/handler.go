package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/service"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, roleMw *jwt.RoleMiddleware)
	CreateClassroom(w http.ResponseWriter, r *http.Request)
	GetClassrooms(w http.ResponseWriter, r *http.Request)
	GetMyClassrooms(w http.ResponseWriter, r *http.Request)
	GetClassroom(w http.ResponseWriter, r *http.Request)
	GetClassroomByCode(w http.ResponseWriter, r *http.Request)
	UpdateClassroom(w http.ResponseWriter, r *http.Request)
	ArchiveClassroom(w http.ResponseWriter, r *http.Request)
	RequestJoinClassroom(w http.ResponseWriter, r *http.Request)
	GetJoinRequests(w http.ResponseWriter, r *http.Request)
	ApproveMember(w http.ResponseWriter, r *http.Request)
	RejectMember(w http.ResponseWriter, r *http.Request)
	RemoveMember(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(service service.Service) Handler {
	return &handler{service: service}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	mux.HandleFunc("POST /classrooms", mw.Handle(h.CreateClassroom))
	mux.HandleFunc("GET /classrooms", mw.Handle(rm.RequireRole("admin")(h.GetClassrooms)))
	mux.HandleFunc("GET /classrooms/me", mw.Handle(h.GetMyClassrooms))
	mux.HandleFunc("GET /classrooms/code/{code}", mw.Handle(h.GetClassroomByCode))
	mux.HandleFunc("GET /classrooms/{id}", mw.Handle(h.GetClassroom))
	mux.HandleFunc("PATCH /classrooms/{id}", mw.Handle(h.UpdateClassroom))
	mux.HandleFunc("POST /classrooms/{id}/archive", mw.Handle(h.ArchiveClassroom))
	mux.HandleFunc("POST /classrooms/join", mw.Handle(h.RequestJoinClassroom))
	mux.HandleFunc("GET /classrooms/{id}/members/join-requests", mw.Handle(h.GetJoinRequests))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/approve", mw.Handle(h.ApproveMember))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/reject", mw.Handle(h.RejectMember))
	mux.HandleFunc("DELETE /classrooms/{id}/members/{userId}", mw.Handle(h.RemoveMember))
}

// CreateClassroom godoc
// @Summary      Create classroom
// @Description  Create a new classroom for the current authenticated user.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateClassroomRequest true "Create classroom request"
// @Success      201 {object} dto.CreatedClassroomResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms [post]
func (h *handler) CreateClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var body dto.CreateClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}

	resp, err := h.service.CreateClassroom(r.Context(), userID, body)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Param        query query string false "Search by classroom code or name"
// @Success      200 {object} httpx.PaginatedResponse[dto.ClassroomResponse]
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms [get]
func (h *handler) GetClassrooms(w http.ResponseWriter, r *http.Request) {
	var query dto.ClassroomsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()

	resp, pageCounts, err := h.service.GetClassrooms(r.Context(), query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, pageCounts))
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
// @Param        query query string false "Search by classroom code or name"
// @Success      200 {object} httpx.PaginatedResponse[dto.ClassroomResponse]
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/me [get]
func (h *handler) GetMyClassrooms(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var query dto.ClassroomsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()

	resp, pageCounts, err := h.service.GetMyClassrooms(r.Context(), userID, query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, pageCounts))
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
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id} [get]
func (h *handler) GetClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}

	resp, err := h.service.GetClassroom(r.Context(), userID, id)
	if err != nil {
		switch err {
		case service.ErrClassroomForbidden:
			httpx.Error(w, http.StatusForbidden, err.Error())
		default:
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		}
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
// @Param        code path string true "Classroom code"
// @Success      200 {object} dto.ClassroomByCodeResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/code/{code} [get]
func (h *handler) GetClassroomByCode(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom code")
		return
	}

	resp, err := h.service.GetClassroomByCode(r.Context(), code)
	if err != nil {
		if err == service.ErrClassroomNotActive {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
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
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id} [patch]
func (h *handler) UpdateClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}

	var body dto.UpdateClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	resp, err := h.service.UpdateClassroom(r.Context(), userID, id, body)
	if err != nil {
		switch err {
		case service.ErrClassroomForbidden:
			httpx.Error(w, http.StatusForbidden, err.Error())
		case service.ErrClassroomNotActive:
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		}
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
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id}/archive [post]
func (h *handler) ArchiveClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}

	resp, err := h.service.ArchiveClassroom(r.Context(), userID, id)
	if err != nil {
		if err == service.ErrClassroomForbidden {
			httpx.Error(w, http.StatusForbidden, err.Error())
			return
		}
		httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RequestJoinClassroom godoc
// @Summary      Request to join classroom
// @Description  Send a join request to an active classroom by code.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.RequestJoinClassroomRequest true "Request join classroom"
// @Success      201 {object} dto.ClassroomJoinRequestResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/join [post]
func (h *handler) RequestJoinClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var body dto.RequestJoinClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	resp, err := h.service.RequestJoinClassroom(r.Context(), userID, body)
	if err != nil {
		switch err {
		case service.ErrClassroomCapacityFull, service.ErrTeacherCannotJoinClass, service.ErrClassroomNotActive:
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// GetJoinRequests godoc
// @Summary      Get classroom join requests
// @Description  Get pending join requests in a classroom. Only the owner teacher can view this list.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Success      200 {array} dto.ClassroomJoinRequestResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id}/members/join-requests [get]
func (h *handler) GetJoinRequests(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}

	resp, err := h.service.GetJoinRequests(r.Context(), userID, id)
	if err != nil {
		if err == service.ErrClassroomForbidden {
			httpx.Error(w, http.StatusForbidden, err.Error())
			return
		}
		httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// ApproveMember godoc
// @Summary      Approve classroom member
// @Description  Approve a pending student join request. Only the owner teacher can approve.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        userId path string true "Student user ID"
// @Success      200 {object} dto.ClassroomJoinRequestResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id}/members/{userId}/approve [post]
func (h *handler) ApproveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid user ID")
		return
	}

	resp, err := h.service.ApproveMember(r.Context(), userID, id, memberID)
	if err != nil {
		switch err {
		case service.ErrClassroomForbidden:
			httpx.Error(w, http.StatusForbidden, err.Error())
		case service.ErrClassroomNotActive:
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		}
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RejectMember godoc
// @Summary      Reject classroom member
// @Description  Reject a pending student join request. Only the owner teacher can reject.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        userId path string true "Student user ID"
// @Success      200 {object} dto.ClassroomJoinRequestResponse
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id}/members/{userId}/reject [post]
func (h *handler) RejectMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid user ID")
		return
	}

	resp, err := h.service.RejectMember(r.Context(), userID, id, memberID)
	if err != nil {
		switch err {
		case service.ErrClassroomForbidden:
			httpx.Error(w, http.StatusForbidden, err.Error())
		case service.ErrClassroomNotActive:
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		}
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// RemoveMember godoc
// @Summary      Remove classroom member
// @Description  Remove a student from classroom by soft deleting the member record. Only the owner teacher can remove.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        userId path string true "Student user ID"
// @Success      204
// @Failure      400 {object} httpx.ErrorResponse
// @Failure      401 {object} httpx.ErrorResponse
// @Failure      403 {object} httpx.ErrorResponse
// @Failure      404 {object} httpx.ErrorResponse
// @Failure      500 {object} httpx.ErrorResponse
// @Router       /classrooms/{id}/members/{userId} [delete]
func (h *handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid classroom ID")
		return
	}
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid user ID")
		return
	}
	if err := h.service.RemoveMember(r.Context(), userID, id, memberID); err != nil {
		if err == service.ErrClassroomForbidden {
			httpx.Error(w, http.StatusForbidden, err.Error())
			return
		}
		httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
