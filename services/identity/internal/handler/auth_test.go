package handler

import (
	"bytes"
	"ego/platform/firebase"
	identityConfig "ego/services/identity/config"
	"ego/services/identity/internal/service"
	"ego/services/identity/internal/token"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefreshMalformedTokenReturnsUnauthorized(t *testing.T) {
	jwtManager := token.NewManager(&identityConfig.JwtConfig{
		JwtSecret:          "test-secret",
		AccessTokenExpiry:  time.Minute,
		RefreshTokenExpiry: time.Hour,
	})
	svc := service.New(jwtManager, firebase.Client{}, nil, nil)
	h := New(svc)

	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBufferString(`{"refreshToken":"not-a-jwt"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	h.Refresh(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, res.Code, res.Body.String())
	}
}
