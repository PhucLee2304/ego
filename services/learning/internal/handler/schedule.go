package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// CreateSchedule godoc
// @Summary      Create a class schedule with its room
// @Tags         Schedules
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        body  body  dto.CreateScheduleRequest  true  "Request body"
// @Success      201  {object}  dto.CalendarResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/schedules [post]
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

// GetCalendar godoc
// @Summary      Get classroom calendar in a time range
// @Tags         Schedules
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        query  query  dto.CalendarQuery  true  "Query"
// @Success      200  {array}  dto.CalendarResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/calendar [get]
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

// UpdateSchedule godoc
// @Summary      Update a class schedule
// @Tags         Schedules
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        scheduleId  path  int  true  "Schedule ID"
// @Param        body  body  dto.UpdateScheduleRequest  true  "Request body"
// @Success      200  {object}  dto.CalendarResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/schedules/{scheduleId} [patch]
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

// DeleteSchedule godoc
// @Summary      Delete a class schedule and its room
// @Tags         Schedules
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Classroom ID"
// @Param        scheduleId  path  int  true  "Schedule ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Router       /classrooms/{id}/schedules/{scheduleId} [delete]
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
