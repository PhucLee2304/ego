package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRoleReturnsUnauthorizedForUnknownUser(t *testing.T) {
	middleware := NewRoleMiddleware(func(context.Context, string) (string, error) {
		return "", ErrUserNotFound
	})
	handler := middleware.RequireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run for an unknown user")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(context.WithValue(request.Context(), userIDKey, "1"))
	recorder := httptest.NewRecorder()
	handler(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}
