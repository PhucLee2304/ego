package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/jwt"
	platformsocket "ego/platform/socket"
)

// fakeIdentityClient chỉ để socket.Server có client khác nil; request không có
// token bị từ chối trước khi gọi tới identity.
type fakeIdentityClient struct {
	identityClient.IdentityServiceClient
}

func newTestMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	authMiddleware := jwt.NewAuthMiddleware(nil)
	roleMiddleware := jwt.NewRoleMiddleware(func(context.Context, string) (string, error) {
		return "admin", nil
	})

	New(nil).RegisterRoutes(mux, authMiddleware, roleMiddleware)
	identity := fakeIdentityClient{}
	RegisterSockets(mux, platformsocket.NewServer(identity), platformsocket.NewServer(identity))
	return mux
}

func TestRegisterRoutesDoesNotPanic(t *testing.T) {
	newTestMux(t)
}

func TestSocketsRequireToken(t *testing.T) {
	mux := newTestMux(t)

	for _, path := range []string{"/ws/exams", "/ws/classrooms"} {
		t.Run(path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("GET %s status = %d, want 401", path, rec.Code)
			}
		})
	}
}

// Route cần đăng nhập: request không có token phải bị middleware chặn (401),
// không được rơi vào 404/405 (route chưa mount).
func TestProtectedRoutesRequireToken(t *testing.T) {
	mux := newTestMux(t)

	for _, route := range []struct{ method, path string }{
		// Classroom
		{http.MethodPost, "/classrooms"},
		{http.MethodGet, "/classrooms"},
		{http.MethodGet, "/classrooms/me"},
		{http.MethodGet, "/classrooms/code"},
		{http.MethodGet, "/classrooms/1"},
		{http.MethodPatch, "/classrooms/1"},
		{http.MethodPost, "/classrooms/1/archive"},
		// Member
		{http.MethodPost, "/classrooms/join"},
		{http.MethodGet, "/classrooms/1/members/join-requests"},
		{http.MethodPost, "/classrooms/1/members/u1/approve"},
		{http.MethodPost, "/classrooms/1/members/u1/reject"},
		{http.MethodDelete, "/classrooms/1/members/u1"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(route.method, route.path, nil))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
		})
	}
}
