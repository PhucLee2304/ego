package handler

import (
	"errors"
	"io"
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/notifications/internal/dto"
)

// @Summary Get current user notifications
// @Description Paginated notifications of the current user (newest first), with total and per-category unread counts.
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page"
// @Param pageSize query int false "Page size"
// @Param read query bool false "Read status"
// @Param category query string false "POSTS | CLASSROOMS | ASSIGNMENTS | ROOMS | EXAMS"
// @Success 200 {object} dto.NotificationsResponse
// @Failure 400 {object} httpx.HTTPError
// @Failure 401 {object} httpx.HTTPError
// @Failure 500 {object} httpx.HTTPError
// @Router /notifications [get]
func (h *handler) GetNotifications(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.NotificationsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()
	response, httpErr := h.service.GetNotifications(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// @Summary Mark selected or all notifications as read
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.MarkNotificationsReadRequest false "Empty IDs mark all as read"
// @Success 204
// @Failure 400 {object} httpx.HTTPError
// @Failure 401 {object} httpx.HTTPError
// @Failure 500 {object} httpx.HTTPError
// @Router /notifications/read [put]
func (h *handler) MarkRead(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.MarkNotificationsReadRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.MarkRead(r.Context(), userID, req.IDs); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Delete selected notifications
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.DeleteNotificationsRequest true "Notification IDs"
// @Success 204
// @Failure 400 {object} httpx.HTTPError
// @Failure 401 {object} httpx.HTTPError
// @Failure 500 {object} httpx.HTTPError
// @Router /notifications [delete]
func (h *handler) DeleteNotifications(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.DeleteNotificationsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.DeleteNotifications(r.Context(), userID, req.IDs); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
