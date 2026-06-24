package jwt

import (
	"context"
	"net/http"
	"strings"

	"ego/api/gen/go/token"
	"ego/platform/httpx"
)

type contextKey string

const userIDKey contextKey = "userID"

type AuthMiddleware struct {
	client token.TokenServiceClient
}

func NewAuthMiddleware(client token.TokenServiceClient) *AuthMiddleware {
	return &AuthMiddleware{client: client}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpx.Error(w, http.StatusUnauthorized, "[JWT] Authorization header is required")
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
			httpx.Error(w, http.StatusUnauthorized, "[JWT] Invalid authorization header format")
			return
		}

		resp, err := m.client.ValidateToken(r.Context(), &token.ValidateTokenRequest{
			Token: bearerToken[1],
		})
		if err != nil || !resp.IsValid {
			httpx.Error(w, http.StatusUnauthorized, "[JWT] Invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, resp.UserId)
		reqWithCtx := r.WithContext(ctx)

		userID, ok := GetUserID(reqWithCtx.Context())
		if !ok || userID == "" {
			httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
			return
		}

		next(w, reqWithCtx)
	}
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}

type RoleResolver func(ctx context.Context, userID string) (string, error)

type RoleMiddleware struct {
	roleResolver RoleResolver
}

func NewRoleMiddleware(roleResolver RoleResolver) *RoleMiddleware {
	return &RoleMiddleware{
		roleResolver: roleResolver,
	}
}

func (m *RoleMiddleware) RequireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	mp := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		mp[role] = struct{}{}
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetUserID(r.Context())
			if !ok || userID == "" {
				httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
				return
			}

			role, err := m.roleResolver(r.Context(), userID)
			if err != nil {
				httpx.Error(w, http.StatusInternalServerError, "[JWT] Failed to resolve role")
				return
			}

			if _, ok := mp[role]; !ok {
				httpx.Error(w, http.StatusForbidden, "[JWT] Forbidden: Access denied")
				return
			}

			next(w, r)
		}
	}
}
