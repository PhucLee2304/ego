package handler

import (
	"errors"
	"io"
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/service"
)

type Handler struct{ service service.Service }

func New(service service.Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("POST /notifications/device-tokens", mw.Handle(h.RegisterDeviceToken))
	mux.HandleFunc("DELETE /notifications/device-tokens", mw.Handle(h.DeleteDeviceToken))
	mux.HandleFunc("GET /notifications", mw.Handle(h.GetNotifications))
	mux.HandleFunc("PATCH /notifications/read", mw.Handle(h.MarkRead))
	mux.HandleFunc("DELETE /notifications", mw.Handle(h.DeleteNotifications))
}

// RegisterDeviceToken godoc
// @Summary Register a device token
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Param request body dto.RegisterDeviceTokenRequest true "Device token"
// @Success 204
// @Failure 400 {object} httpx.ErrorResponse
// @Router /notifications/device-tokens [post]
func (h *Handler) RegisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.RegisterDeviceTokenRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.RegisterDeviceToken(r.Context(), userID, req); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteDeviceToken godoc
// @Summary Delete the current device token
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Param request body dto.DeleteDeviceTokenRequest true "Device token"
// @Success 204
// @Router /notifications/device-tokens [delete]
func (h *Handler) DeleteDeviceToken(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.DeleteDeviceTokenRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.DeleteDeviceToken(r.Context(), userID, req.Token); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetNotifications godoc
// @Summary Get current user notifications
// @Tags Notifications
// @Security BearerAuth
// @Param page query int false "Page"
// @Param pageSize query int false "Page size"
// @Param read query bool false "Read status"
// @Success 200 {object} dto.NotificationsResponse
// @Router /notifications [get]
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.NotificationsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()
	response, err := h.service.GetNotifications(r.Context(), userID, query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// MarkRead godoc
// @Summary Mark selected or all notifications as read
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Param request body dto.NotificationIDsRequest false "Empty IDs mark all as read"
// @Success 204
// @Router /notifications/read [patch]
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.NotificationIDsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.MarkRead(r.Context(), userID, req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteNotifications godoc
// @Summary Delete selected notifications
// @Tags Notifications
// @Security BearerAuth
// @Accept json
// @Param request body dto.DeleteNotificationsRequest true "Notification IDs"
// @Success 204
// @Router /notifications [delete]
func (h *Handler) DeleteNotifications(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.DeleteNotificationsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.DeleteNotifications(r.Context(), userID, req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
