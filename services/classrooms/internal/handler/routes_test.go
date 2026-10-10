package handler

import (
	"context"
	"net/http"
	"testing"

	"ego/platform/jwt"
)

func TestRegisterRoutesDoesNotPanic(t *testing.T) {
	mux := http.NewServeMux()
	authMiddleware := jwt.NewAuthMiddleware(nil)
	roleMiddleware := jwt.NewRoleMiddleware(func(context.Context, string) (string, error) {
		return "admin", nil
	})

	New(nil).RegisterRoutes(mux, authMiddleware, roleMiddleware)
}
