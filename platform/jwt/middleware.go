package jwt

import (
	"context"
	"net/http"
	"strings"

	"ego/api/gen/go/token"
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
			http.Error(w, "[JWT] Authorization header is required", http.StatusUnauthorized)
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
			http.Error(w, "[JWT] Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		resp, err := m.client.ValidateToken(r.Context(), &token.ValidateTokenRequest{
			Token: bearerToken[1],
		})
		if err != nil || !resp.IsValid {
			http.Error(w, "[JWT] Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, resp.UserId)
		next(w, r.WithContext(ctx))
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
				http.Error(w, "[JWT] Unauthorized", http.StatusUnauthorized)
				return
			}

			role, err := m.roleResolver(r.Context(), userID)
			if err != nil {
				http.Error(w, "[JWT] Failed to resolve role", http.StatusInternalServerError)
				return
			}

			if _, ok := mp[role]; !ok {
				http.Error(w, "[JWT] Forbidden: Access denied", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}
