package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	GetStatus(w http.ResponseWriter, r *http.Request)
}

type handler struct{}

func New() Handler {
	return &handler{}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("GET /classrooms/status", mw.Handle(h.GetStatus))
}

// GetStatus godoc
// @Summary      Get classrooms service status
// @Description  Get authenticated classrooms service status
// @Tags         Classrooms
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]string
// @Failure      401  {object}  httpx.ErrorResponse
// @Router       /classrooms/status [get]
func (h *handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
}
