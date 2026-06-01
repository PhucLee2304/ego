package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/auth/internal/dto"
	"ego/services/auth/internal/service"
	"errors"
	"fmt"
	"net/http"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware)
	Refresh(w http.ResponseWriter, r *http.Request)
	Login(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	g := func(pattern string, handlerFunc http.HandlerFunc) {
		mux.HandleFunc(pattern, mw.Handle(handlerFunc))
	}

	g("POST /refresh", h.Refresh)
	mux.HandleFunc("POST /login", h.Login)
}

// Refresh godoc
// @Summary      Refresh access token
// @Description  Refresh an access token using a refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.RefreshBody true "Refresh Request"
// @Success      200  {object}  dto.RefreshResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /refresh [post]
func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body dto.RefreshBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}

	accessToken, refreshToken, err := h.service.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		if errors.Is(err, jwt.ErrInvalidToken) {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] Invalid refresh token")
		} else if errors.Is(err, jwt.ErrTokenExpired) {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] Refresh token expired")
		} else {
			httpx.Error(w, http.StatusInternalServerError, "[ERROR] Failed to refresh token")
		}
		return
	}

	httpx.JSON(w, http.StatusOK, dto.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

// Login godoc
// @Summary      Login with Firebase ID Token
// @Description  Exchange a Firebase ID token for an access and refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginBody true "Login Request"
// @Success      200  {object}  dto.LoginResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /login [post]
func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var body dto.LoginBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}

	resp, err := h.service.Login(r.Context(), body)
	if err != nil {
		fmt.Printf("Login error: %v\n", err)
		if errors.Is(err, jwt.ErrInvalidToken) || errors.Is(err, service.ErrInvalidIdToken) {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] Invalid token")
		} else if errors.Is(err, jwt.ErrTokenExpired) {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] Token expired")
		} else {
			httpx.Error(w, http.StatusInternalServerError, "[ERROR] Failed to login: "+err.Error())
		}
		return
	}

	httpx.JSON(w, http.StatusOK, dto.LoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}
