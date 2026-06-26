package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/storage/internal/dto"
	"ego/services/storage/internal/service"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	PresignUploadURL(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	svc service.Service
}

func New(svc service.Service) Handler {
	return &handler{svc: svc}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("POST /presign/upload", mw.Handle(h.PresignUploadURL))
}

// @Summary Generate Presigned URL for Uploading File
// @Description Generates a presigned URL for uploading a file to storage
// @Tags Storage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.PresignUploadRequest true "Request Body"
// @Success 200 {object} dto.PresignUploadResponse
// @Failure 400 {object} httpx.ErrorResponse
// @Failure 500 {object} httpx.ErrorResponse
// @Router /presign/upload [post]
func (h *handler) PresignUploadURL(w http.ResponseWriter, r *http.Request) {
	var req dto.PresignUploadRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.svc.GeneratePresignUploadURL(r.Context(), req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
