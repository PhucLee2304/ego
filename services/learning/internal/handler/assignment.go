package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// CreateAssignments godoc
// @Summary      Create classroom assignments
// @Description  Create a private exam and assign it to multiple classrooms owned by the current teacher.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateAssignmentsRequest true "Create assignments request"
// @Success      201 {object} dto.CreatedAssignmentsResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/assignments [post]
func (h *handler) CreateAssignments(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var body dto.CreateAssignmentsRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.CreateAssignments(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// GetAssignments godoc
// @Summary      Get classroom assignments
// @Description  Get assignments when the current user is the teacher or an approved student.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200 {object} httpx.PaginatedResponse[dto.AssignmentResponse]
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments [get]
func (h *handler) GetAssignments(w http.ResponseWriter, r *http.Request) {
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

	var query dto.AssignmentsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	resp, total, httpErr := h.service.GetAssignments(r.Context(), userID, id, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, total))
}

// GetAssignment godoc
// @Summary      Get classroom assignment
// @Description  Get an assignment when the current user is the teacher or an approved student.
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
// @Router       /classrooms/{id}/assignments/{assignmentId} [get]
func (h *handler) GetAssignment(w http.ResponseWriter, r *http.Request) {
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

	resp, httpErr := h.service.GetAssignment(r.Context(), userID, id, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *handler) AssignExistingExam(w http.ResponseWriter, r *http.Request) {
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
	var body dto.AssignExistingExamRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	resp, httpErr := h.service.AssignExistingExam(r.Context(), userID, classroomID, assignmentID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

func (h *handler) UpdateAssignmentSchedule(w http.ResponseWriter, r *http.Request) {
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
	var body dto.UpdateAssignmentScheduleRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	resp, httpErr := h.service.UpdateAssignmentSchedule(r.Context(), userID, classroomID, assignmentID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// SetAssignmentActive godoc
// @Summary      Activate or deactivate an assignment
// @Description  Update assignment availability. Only the classroom teacher can perform this action.
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Classroom ID"
// @Param        assignmentId path int true "Assignment ID"
// @Param        request body dto.SetAssignmentActiveRequest true "Assignment active state"
// @Success      200 {object} dto.AssignmentResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      404 {object} httpx.HTTPError
// @Router       /classrooms/{id}/assignments/{assignmentId}/active [patch]
func (h *handler) SetAssignmentActive(w http.ResponseWriter, r *http.Request) {
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
	var body dto.SetAssignmentActiveRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.SetAssignmentActive(r.Context(), userID, classroomID, assignmentID, *body.Active)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

func (h *handler) GetEditableAssignment(w http.ResponseWriter, r *http.Request) {
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
	response, httpErr := h.service.GetEditableAssignment(r.Context(), userID, classroomID, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) UpdateAssignmentContent(w http.ResponseWriter, r *http.Request) {
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
	var body dto.UpdateAssignmentContentRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.UpdateAssignmentContent(r.Context(), userID, classroomID, assignmentID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) ResetAssignmentAttempt(w http.ResponseWriter, r *http.Request) {
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
	studentID := r.PathValue("studentId")
	if studentID == "" {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid student ID", nil))
		return
	}
	if httpErr := h.service.ResetAssignmentAttempt(r.Context(), userID, classroomID, assignmentID, studentID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) GetAssignmentAudits(w http.ResponseWriter, r *http.Request) {
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
	response, httpErr := h.service.GetAssignmentAudits(r.Context(), userID, classroomID, assignmentID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
