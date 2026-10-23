package handler

import (
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"net/http"
)

// Refresh godoc
// @Summary      Refresh access token
// @Description  Refresh an access token using a refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.RefreshBody true "Refresh Request"
// @Success      200  {object}  dto.RefreshResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /refresh [post]
func (h *handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body dto.RefreshBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	accessToken, refreshToken, httpErr := h.service.Refresh(r.Context(), body.RefreshToken)
	if httpErr != nil {
		httpx.Error(w, httpErr)
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
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /login [post]
func (h *handler) Login(w http.ResponseWriter, r *http.Request) {
	var body dto.LoginBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.Login(r.Context(), body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, dto.LoginResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	})
}
