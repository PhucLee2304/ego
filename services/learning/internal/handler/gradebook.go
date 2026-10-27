package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// GetGradebook godoc
// @Summary      Get assignment gradebook
// @Description  Get submissions and scores for one classroom assignment. Only the owner teacher can view gradebook.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Success      200 {object} dto.AssignmentResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/gradebook [get]
func (h *handler) GetGradebook(w http.ResponseWriter, r *http.Request) {
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
	assignmentID, err := httpx.ParsePathID(r, "assignmentId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid assignment ID", err))
		return
	}

	resp, httpErr := h.service.GetGradebook(r.Context(), userID, id, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// GetClassGradebook godoc
// @Summary Get class gradebook
// @Tags Gradebook
// @Security BearerAuth
// @Param id path int true "Classroom ID"
// @Param from query string false "Minimum assignment due time (RFC3339)"
// @Param to query string false "Maximum assignment due time (RFC3339, exclusive)"
// @Success 200 {object} dto.ClassGradebookResponse
// @Router /classrooms/{id}/gradebook [get]
func (h *handler) GetClassGradebook(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}
	var query dto.GradebookRangeQuery
	if err := httpx.DecodeQuery(r, &query); err != nil || (query.From != nil && query.To != nil && !query.To.After(*query.From)) {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid gradebook range", err))
		return
	}
	response, httpErr := h.service.GetClassGradebook(r.Context(), userID, classroomID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// GetStudentProgress godoc
// @Summary Get student assignment progress
// @Tags Gradebook
// @Security BearerAuth
// @Param id path int true "Classroom ID"
// @Param userId path string true "Student ID"
// @Success 200 {object} dto.StudentProgressResponse
// @Router /classrooms/{id}/students/{userId}/progress [get]
func (h *handler) GetStudentProgress(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}
	studentID := r.PathValue("userId")
	if studentID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid user ID", nil))
		return
	}
	response, httpErr := h.service.GetStudentProgress(r.Context(), userID, classroomID, studentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// GetAssignmentAnalytics godoc
// @Summary Get assignment analytics
// @Tags Gradebook
// @Security BearerAuth
// @Param id path int true "Classroom ID"
// @Param assignmentId path int true "Assignment ID"
// @Success 200 {object} dto.AssignmentAnalyticsResponse
// @Router /classrooms/{id}/assignments/{assignmentId}/analytics [get]
func (h *handler) GetAssignmentAnalytics(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}
	assignmentID, err := httpx.ParsePathID(r, "assignmentId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid assignment ID", err))
		return
	}
	response, httpErr := h.service.GetAssignmentAnalytics(r.Context(), userID, classroomID, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// ReconcileAssignmentSubmissions godoc
// @Summary      Reconcile assignment submissions
// @Description  Re-sync submitted attempt scores into the assignment gradebook. Only the owner teacher can reconcile.
// @Tags         Gradebook
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Success      202 {object} map[string]int
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/gradebook/reconcile [post]
func (h *handler) ReconcileAssignmentSubmissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return
	}
	assignmentID, err := httpx.ParsePathID(r, "assignmentId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid assignment ID", err))
		return
	}
	count, httpErr := h.service.ReconcileAssignmentSubmissions(r.Context(), userID, classroomID, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]int{"enqueuedCount": count})
}
