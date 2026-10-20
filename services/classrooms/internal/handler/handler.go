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
	CreateAssignments(w http.ResponseWriter, r *http.Request)
	AssignExistingExam(w http.ResponseWriter, r *http.Request)
	UpdateAssignmentSchedule(w http.ResponseWriter, r *http.Request)
	GetAssignments(w http.ResponseWriter, r *http.Request)
	GetAssignment(w http.ResponseWriter, r *http.Request)
	SetAssignmentActive(w http.ResponseWriter, r *http.Request)
	GetGradebook(w http.ResponseWriter, r *http.Request)
	GetClassGradebook(w http.ResponseWriter, r *http.Request)
	GetStudentProgress(w http.ResponseWriter, r *http.Request)
	GetAssignmentAnalytics(w http.ResponseWriter, r *http.Request)
	GetEditableAssignment(w http.ResponseWriter, r *http.Request)
	UpdateAssignmentContent(w http.ResponseWriter, r *http.Request)
	ResetAssignmentAttempt(w http.ResponseWriter, r *http.Request)
	ReconcileAssignmentSubmissions(w http.ResponseWriter, r *http.Request)
	GetAssignmentAudits(w http.ResponseWriter, r *http.Request)
	CreateSchedule(w http.ResponseWriter, r *http.Request)
	GetCalendar(w http.ResponseWriter, r *http.Request)
	UpdateSchedule(w http.ResponseWriter, r *http.Request)
	DeleteSchedule(w http.ResponseWriter, r *http.Request)
	GetRoom(w http.ResponseWriter, r *http.Request)
	GetRoomActivity(w http.ResponseWriter, r *http.Request)
	StartRoom(w http.ResponseWriter, r *http.Request)
	JoinRoom(w http.ResponseWriter, r *http.Request)
	EndRoom(w http.ResponseWriter, r *http.Request)
	GetRoomAttendance(w http.ResponseWriter, r *http.Request)
	RequestScreenShare(w http.ResponseWriter, r *http.Request)
	ApproveScreenShare(w http.ResponseWriter, r *http.Request)
	RejectScreenShare(w http.ResponseWriter, r *http.Request)
	StartScreenShare(w http.ResponseWriter, r *http.Request)
	StopScreenShare(w http.ResponseWriter, r *http.Request)
	SetHandRaised(w http.ResponseWriter, r *http.Request)
	LowerHands(w http.ResponseWriter, r *http.Request)
	MuteParticipant(w http.ResponseWriter, r *http.Request)
	AllowParticipantMicrophone(w http.ResponseWriter, r *http.Request)
	DisableParticipantCamera(w http.ResponseWriter, r *http.Request)
	AllowParticipantCamera(w http.ResponseWriter, r *http.Request)
	MuteAllParticipants(w http.ResponseWriter, r *http.Request)
	RemoveParticipant(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service *service.HTTPService
}

func New(classroomService service.Service) Handler {
	return &handler{service: service.NewHTTPService(classroomService)}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	mux.HandleFunc("POST /classrooms", mw.Handle(h.CreateClassroom))
	mux.HandleFunc("GET /classrooms", mw.Handle(rm.RequireRole("admin")(h.GetClassrooms)))
	mux.HandleFunc("GET /classrooms/me", mw.Handle(h.GetMyClassrooms))
	mux.HandleFunc("GET /classrooms/code", mw.Handle(h.GetClassroomByCode))
	mux.HandleFunc("GET /classrooms/{id}", mw.Handle(h.GetClassroom))
	mux.HandleFunc("PATCH /classrooms/{id}", mw.Handle(h.UpdateClassroom))
	mux.HandleFunc("POST /classrooms/{id}/archive", mw.Handle(h.ArchiveClassroom))
	mux.HandleFunc("POST /classrooms/join", mw.Handle(h.RequestJoinClassroom))
	mux.HandleFunc("GET /classrooms/{id}/members/join-requests", mw.Handle(h.GetJoinRequests))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/approve", mw.Handle(h.ApproveMember))
	mux.HandleFunc("POST /classrooms/{id}/members/{userId}/reject", mw.Handle(h.RejectMember))
	mux.HandleFunc("DELETE /classrooms/{id}/members/{userId}", mw.Handle(h.RemoveMember))
	mux.HandleFunc("POST /classrooms/assignments", mw.Handle(h.CreateAssignments))
	mux.HandleFunc("GET /classrooms/{id}/assignments", mw.Handle(h.GetAssignments))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}", mw.Handle(h.GetAssignment))
	mux.HandleFunc("POST /classrooms/{id}/assignments/{assignmentId}/classrooms", mw.Handle(h.AssignExistingExam))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}/schedule", mw.Handle(h.UpdateAssignmentSchedule))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}/active", mw.Handle(h.SetAssignmentActive))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/gradebook", mw.Handle(h.GetGradebook))
	mux.HandleFunc("GET /classrooms/{id}/gradebook", mw.Handle(h.GetClassGradebook))
	mux.HandleFunc("GET /classrooms/{id}/students/{userId}/progress", mw.Handle(h.GetStudentProgress))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/analytics", mw.Handle(h.GetAssignmentAnalytics))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/edit", mw.Handle(h.GetEditableAssignment))
	mux.HandleFunc("PATCH /classrooms/{id}/assignments/{assignmentId}", mw.Handle(h.UpdateAssignmentContent))
	mux.HandleFunc("DELETE /classrooms/{id}/assignments/{assignmentId}/students/{studentId}/attempt", mw.Handle(h.ResetAssignmentAttempt))
	mux.HandleFunc("POST /classrooms/{id}/assignments/{assignmentId}/gradebook/reconcile", mw.Handle(h.ReconcileAssignmentSubmissions))
	mux.HandleFunc("GET /classrooms/{id}/assignments/{assignmentId}/audits", mw.Handle(h.GetAssignmentAudits))
	mux.HandleFunc("POST /classrooms/{id}/schedules", mw.Handle(h.CreateSchedule))
	mux.HandleFunc("GET /classrooms/{id}/calendar", mw.Handle(h.GetCalendar))
	mux.HandleFunc("PATCH /classrooms/{id}/schedules/{scheduleId}", mw.Handle(h.UpdateSchedule))
	mux.HandleFunc("DELETE /classrooms/{id}/schedules/{scheduleId}", mw.Handle(h.DeleteSchedule))
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}", mw.Handle(h.GetRoom))
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}/activity", mw.Handle(h.GetRoomActivity))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/start", mw.Handle(h.StartRoom))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/join", mw.Handle(h.JoinRoom))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/end", mw.Handle(h.EndRoom))
	mux.HandleFunc("GET /classrooms/{id}/rooms/{roomId}/attendance", mw.Handle(h.GetRoomAttendance))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/request", mw.Handle(h.RequestScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/start", mw.Handle(h.StartScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/approve", mw.Handle(h.ApproveScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/reject", mw.Handle(h.RejectScreenShare))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/screen-share/{userId}/stop", mw.Handle(h.StopScreenShare))
	mux.HandleFunc("PATCH /classrooms/{id}/rooms/{roomId}/hand", mw.Handle(h.SetHandRaised))
	mux.HandleFunc("DELETE /classrooms/{id}/rooms/{roomId}/hands", mw.Handle(h.LowerHands))
	mux.HandleFunc("DELETE /classrooms/{id}/rooms/{roomId}/hands/{userId}", mw.Handle(h.LowerHands))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/mute-all", mw.Handle(h.MuteAllParticipants))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/mute", mw.Handle(h.MuteParticipant))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-microphone", mw.Handle(h.AllowParticipantMicrophone))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/disable-camera", mw.Handle(h.DisableParticipantCamera))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/allow-camera", mw.Handle(h.AllowParticipantCamera))
	mux.HandleFunc("POST /classrooms/{id}/rooms/{roomId}/participants/{userId}/remove", mw.Handle(h.RemoveParticipant))
}

func (h *handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
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
	var body dto.CreateScheduleRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreateSchedule(r.Context(), userID, classroomID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *handler) GetCalendar(w http.ResponseWriter, r *http.Request) {
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
	var query dto.CalendarQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid calendar range", err))
		return
	}
	response, httpErr := h.service.GetCalendar(r.Context(), userID, classroomID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	scheduleID, err := httpx.ParsePathID(r, "scheduleId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid schedule ID", err))
		return
	}
	var body dto.UpdateScheduleRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.UpdateSchedule(r.Context(), userID, scheduleID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}
	scheduleID, err := httpx.ParsePathID(r, "scheduleId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid schedule ID", err))
		return
	}
	if httpErr := h.service.DeleteSchedule(r.Context(), userID, scheduleID); httpErr != nil {
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

	resp, pageCounts, httpErr := h.service.GetClassrooms(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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

	resp, pageCounts, httpErr := h.service.GetMyClassrooms(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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

	resp, pageCounts, httpErr := h.service.GetAssignments(r.Context(), userID, id, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, pageCounts))
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
