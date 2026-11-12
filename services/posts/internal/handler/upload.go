package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/posts/internal/dto"
)

// GetUploadLimits godoc
// @Summary      Get upload limits
// @Description  Size limits and allowed content types for post attachments
// @Tags         Uploads
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.UploadLimitsResponse
// @Failure      401  {object}  httpx.HTTPError
// @Router       /uploads/config [get]
func (h *handler) GetUploadLimits(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, h.service.UploadLimits())
}

// CreateUploadIntent godoc
// @Summary      Create an upload intent
// @Description  Returns a presigned URL to upload one attachment; call complete after uploading
// @Tags         Uploads
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreateUploadIntentRequest true "Upload intent"
// @Success      201  {object}  dto.UploadIntentResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /uploads [post]
func (h *handler) CreateUploadIntent(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var request dto.CreateUploadIntentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreateUploadIntent(r.Context(), userID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

// CompleteUpload godoc
// @Summary      Complete an upload
// @Description  Verifies the uploaded object and creates an attachment that can be attached to a post
// @Tags         Uploads
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Upload intent ID"
// @Success      200  {object}  dto.AttachmentResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /uploads/{id}/complete [post]
func (h *handler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	response, httpErr := h.service.CompleteUpload(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

// CancelUpload godoc
// @Summary      Cancel an upload
// @Description  Cancels a pending upload intent and deletes the uploaded object
// @Tags         Uploads
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Upload intent ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /uploads/{id} [delete]
func (h *handler) CancelUpload(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.CancelUpload(r.Context(), userID, id); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
