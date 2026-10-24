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

	h := New(nil)
	h.RegisterRoutes(mux, authMiddleware, roleMiddleware)
	identity := fakeIdentityClient{}
	examsHub := platformsocket.NewServer(identity)
	h.RegisterSocketHandlers(examsHub)
	RegisterSockets(mux, examsHub, platformsocket.NewServer(identity))
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
		// Exam
		{http.MethodGet, "/exams"},
		{http.MethodGet, "/exams/1"},
		// Attempt
		{http.MethodPost, "/exams/1/attempts"},
		{http.MethodGet, "/attempts"},
		{http.MethodGet, "/attempts/1/questions"},
		{http.MethodPost, "/attempts/1/submit"},
		{http.MethodPost, "/attempts/1/cancel"},
		{http.MethodGet, "/attempts/1/history"},
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
		// Assignment
		{http.MethodPost, "/classrooms/assignments"},
		{http.MethodGet, "/classrooms/1/assignments"},
		{http.MethodGet, "/classrooms/1/assignments/2"},
		{http.MethodPost, "/classrooms/1/assignments/2/classrooms"},
		{http.MethodPatch, "/classrooms/1/assignments/2/schedule"},
		{http.MethodPatch, "/classrooms/1/assignments/2/active"},
		{http.MethodGet, "/classrooms/1/assignments/2/edit"},
		{http.MethodPatch, "/classrooms/1/assignments/2"},
		{http.MethodDelete, "/classrooms/1/assignments/2/students/u1/attempt"},
		{http.MethodGet, "/classrooms/1/assignments/2/audits"},
		// Schedule
		{http.MethodPost, "/classrooms/1/schedules"},
		{http.MethodGet, "/classrooms/1/calendar"},
		{http.MethodPatch, "/classrooms/1/schedules/2"},
		{http.MethodDelete, "/classrooms/1/schedules/2"},
		// Room
		{http.MethodGet, "/classrooms/1/rooms/2"},
		{http.MethodGet, "/classrooms/1/rooms/2/activity"},
		{http.MethodPost, "/classrooms/1/rooms/2/start"},
		{http.MethodPost, "/classrooms/1/rooms/2/join"},
		{http.MethodPost, "/classrooms/1/rooms/2/end"},
		{http.MethodGet, "/classrooms/1/rooms/2/attendance"},
		// Screen share
		{http.MethodPost, "/classrooms/1/rooms/2/screen-share/request"},
		{http.MethodPost, "/classrooms/1/rooms/2/screen-share/start"},
		{http.MethodPost, "/classrooms/1/rooms/2/screen-share/u1/approve"},
		{http.MethodPost, "/classrooms/1/rooms/2/screen-share/u1/reject"},
		{http.MethodPost, "/classrooms/1/rooms/2/screen-share/u1/stop"},
		// Hands
		{http.MethodPatch, "/classrooms/1/rooms/2/hand"},
		{http.MethodDelete, "/classrooms/1/rooms/2/hands"},
		{http.MethodDelete, "/classrooms/1/rooms/2/hands/u1"},
		// Moderation
		{http.MethodPost, "/classrooms/1/rooms/2/participants/mute-all"},
		{http.MethodPost, "/classrooms/1/rooms/2/participants/u1/mute"},
		{http.MethodPost, "/classrooms/1/rooms/2/participants/u1/allow-microphone"},
		{http.MethodPost, "/classrooms/1/rooms/2/participants/u1/disable-camera"},
		{http.MethodPost, "/classrooms/1/rooms/2/participants/u1/allow-camera"},
		{http.MethodPost, "/classrooms/1/rooms/2/participants/u1/remove"},
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
