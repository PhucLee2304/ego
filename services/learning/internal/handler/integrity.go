package handler

import (
	"net/http"
	"strings"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// Giám sát làm bài phía giáo viên (PLAN_ATTEMPT_INTEGRITY.md mục 1.9).

// integrityTarget đọc user hiện tại, classroom ID và assignment ID từ request.
func integrityTarget(w http.ResponseWriter, r *http.Request) (string, uint, uint, bool) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return "", 0, 0, false
	}
	classroomID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid classroom ID", err))
		return "", 0, 0, false
	}
	assignmentID, err := httpx.ParsePathID(r, "assignmentId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid assignment ID", err))
		return "", 0, 0, false
	}
	return userID, classroomID, assignmentID, true
}

func studentIDParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	studentID := strings.TrimSpace(r.PathValue("studentId"))
	if studentID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid student ID", nil))
		return "", false
	}
	return studentID, true
}

// GetIntegrityEvents godoc
// @Summary      List integrity events of an assignment
// @Description  Teacher lists proctoring events of a proctored assignment; pending=true returns unreviewed violations.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        studentId query string false "Student ID"
// @Param        pending query bool false "Only unreviewed violations"
// @Param        level query string false "LOG, WARNING or VIOLATION"
// @Param        page query int false "Page"
// @Param        pageSize query int false "Page size"
// @Success      200 {object} httpx.PaginatedResponse[dto.AttemptIntegrityEventResponse]
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/integrity-events [get]
func (h *handler) GetIntegrityEvents(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, assignmentID, ok := integrityTarget(w, r)
	if !ok {
		return
	}
	var query dto.IntegrityEventsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()
	response, pageCounts, httpErr := h.service.GetIntegrityEvents(r.Context(), userID, classroomID, assignmentID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(response, query.PaginationQuery, pageCounts))
}

// ReviewIntegrityEvent godoc
// @Summary      Review a violation
// @Description  Teacher lets the student continue (CONTINUE, the violation still counts) or bans the attempt (BAN, score 0). Same action again returns 200; a different action returns 409.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        eventId path string true "Integrity event ID"
// @Param        body body dto.ReviewIntegrityEventRequest true "Review action"
// @Success      200 {object} dto.AttemptIntegrityResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      409 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/integrity-events/{eventId}/review [post]
func (h *handler) ReviewIntegrityEvent(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, assignmentID, ok := integrityTarget(w, r)
	if !ok {
		return
	}
	eventID := strings.TrimSpace(r.PathValue("eventId"))
	if eventID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid event ID", nil))
		return
	}
	var body dto.ReviewIntegrityEventRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.ReviewIntegrityEvent(r.Context(), userID, classroomID, assignmentID, eventID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// BanAttempt godoc
// @Summary      Ban a student's attempt
// @Description  Teacher ends the student's active proctored attempt now with score 0.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        studentId path string true "Student ID"
// @Param        body body dto.BanAttemptRequest false "Optional note"
// @Success      200 {object} dto.AttemptIntegrityResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      409 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/students/{studentId}/attempt/ban [post]
func (h *handler) BanAttempt(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, assignmentID, ok := integrityTarget(w, r)
	if !ok {
		return
	}
	studentID, ok := studentIDParam(w, r)
	if !ok {
		return
	}
	var body dto.BanAttemptRequest
	if r.ContentLength != 0 {
		if err := httpx.DecodeJSON(r, &body); err != nil {
			httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
			return
		}
	}
	response, httpErr := h.service.BanAttempt(r.Context(), userID, classroomID, assignmentID, studentID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// ReleaseAttemptSession godoc
// @Summary      Release a student's attempt session
// @Description  Teacher clears the session owner so the student can continue on another tab or device immediately.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        studentId path string true "Student ID"
// @Success      200 {object} dto.AttemptIntegrityResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/students/{studentId}/attempt/release-session [post]
func (h *handler) ReleaseAttemptSession(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, assignmentID, ok := integrityTarget(w, r)
	if !ok {
		return
	}
	studentID, ok := studentIDParam(w, r)
	if !ok {
		return
	}
	response, httpErr := h.service.ReleaseAttemptSession(r.Context(), userID, classroomID, assignmentID, studentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// ReinstateAttempt godoc
// @Summary      Restore a suspended attempt result
// @Description  Teacher restores the graded score of a suspended attempt; a note is required. The attempt stays submitted.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        studentId path string true "Student ID"
// @Param        body body dto.ReinstateAttemptRequest true "Reason"
// @Success      200 {object} dto.AttemptIntegrityResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      409 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/students/{studentId}/attempt/reinstate [post]
func (h *handler) ReinstateAttempt(w http.ResponseWriter, r *http.Request) {
	userID, classroomID, assignmentID, ok := integrityTarget(w, r)
	if !ok {
		return
	}
	studentID, ok := studentIDParam(w, r)
	if !ok {
		return
	}
	var body dto.ReinstateAttemptRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.ReinstateAttempt(r.Context(), userID, classroomID, assignmentID, studentID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
