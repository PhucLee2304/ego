package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// RequestJoinClassroom godoc
// @Summary      Request to join classroom
// @Description  Send a join request to an active classroom by code.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.RequestJoinClassroomRequest true "Request join classroom"
// @Success      201 {object} dto.ClassroomJoinRequestResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/join [post]
func (h *handler) RequestJoinClassroom(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var body dto.RequestJoinClassroomRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	resp, httpErr := h.service.RequestJoinClassroom(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/members/join-requests [get]
func (h *handler) GetJoinRequests(w http.ResponseWriter, r *http.Request) {
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

	resp, httpErr := h.service.GetJoinRequests(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/members/{userId}/approve [post]
func (h *handler) ApproveMember(w http.ResponseWriter, r *http.Request) {
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
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid user ID", nil))
		return
	}

	resp, httpErr := h.service.ApproveMember(r.Context(), userID, id, memberID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/members/{userId}/reject [post]
func (h *handler) RejectMember(w http.ResponseWriter, r *http.Request) {
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
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid user ID", nil))
		return
	}

	resp, httpErr := h.service.RejectMember(r.Context(), userID, id, memberID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/members/{userId} [delete]
func (h *handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
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
	memberID := r.PathValue("userId")
	if memberID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid user ID", nil))
		return
	}
	if httpErr := h.service.RemoveMember(r.Context(), userID, id, memberID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
