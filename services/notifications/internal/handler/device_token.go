package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/notifications/internal/dto"
)

// @Summary Register a device token
// @Description Binds the push token of this device to the current user. A token already held by another device row is moved to this device.
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegisterDeviceTokenRequest true "Device token"
// @Success 204
// @Failure 400 {object} httpx.HTTPError
// @Failure 401 {object} httpx.HTTPError
// @Failure 409 {object} httpx.HTTPError
// @Failure 500 {object} httpx.HTTPError
// @Router /notifications/device-tokens [post]
func (h *handler) RegisterDeviceToken(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.RegisterDeviceTokenRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.RegisterDeviceToken(r.Context(), userID, req); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary Delete the current device token
// @Tags Notifications
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.DeleteDeviceTokenRequest true "Device token"
// @Success 204
// @Failure 400 {object} httpx.HTTPError
// @Failure 401 {object} httpx.HTTPError
// @Failure 500 {object} httpx.HTTPError
// @Router /notifications/device-tokens [delete]
func (h *handler) DeleteDeviceToken(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	userID, _ := jwt.GetUserID(r.Context())
	var req dto.DeleteDeviceTokenRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.DeleteDeviceToken(r.Context(), userID, req.Token); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
