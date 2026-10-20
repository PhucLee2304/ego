package jwt

import (
	"context"
	"errors"
	"net/http"
	"strings"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/httpx"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type contextKey string

const userIDKey contextKey = "userID"

var ErrInvalidToken = errors.New("token is invalid")

// ErrUserNotFound is returned by a RoleResolver when the user does not exist.
var ErrUserNotFound = errors.New("user not found")

// TokenValidator returns the user ID when the token is valid.
type TokenValidator func(ctx context.Context, token string) (string, error)

type AuthMiddleware struct {
	validate TokenValidator
}

func NewAuthMiddleware(identityServiceClient identityClient.IdentityServiceClient) *AuthMiddleware {
	return NewAuthMiddlewareWithValidator(func(ctx context.Context, tokenString string) (string, error) {
		tokenResp, err := identityServiceClient.ValidateToken(ctx, &identityClient.ValidateTokenRequest{Token: tokenString})
		if err != nil {
			return "", err
		}
		if !tokenResp.IsValid {
			return "", ErrInvalidToken
		}
		return tokenResp.UserId, nil
	})
}

func NewAuthMiddlewareWithValidator(validate TokenValidator) *AuthMiddleware {
	return &AuthMiddleware{validate: validate}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[JWT] Authorization header is required", nil))
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 || strings.ToLower(bearerToken[0]) != "bearer" {
			httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[JWT] Invalid authorization header format", nil))
			return
		}

		userID, err := m.validate(r.Context(), bearerToken[1])
		if err != nil {
			// A gRPC status means identity could not be reached or failed; the
			// token itself was never judged, so do not report it as invalid.
			if _, isRPCError := status.FromError(err); isRPCError {
				httpx.Error(w, httpx.NewError(http.StatusServiceUnavailable, "[JWT] Authentication service unavailable", err))
				return
			}
			httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[JWT] Invalid token", nil))
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, userID)
		reqWithCtx := r.WithContext(ctx)

		if _, ok := GetUserID(reqWithCtx.Context()); !ok {
			httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
			return
		}

		next(w, reqWithCtx)
	}
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok {
		return "", false
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", false
	}

	return userID, true
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
		mp[strings.ToLower(strings.TrimSpace(role))] = struct{}{}
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userID, ok := GetUserID(r.Context())
			if !ok {
				httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
				return
			}

			role, err := m.roleResolver(r.Context(), userID)
			if errors.Is(err, ErrUserNotFound) || errors.Is(err, gorm.ErrRecordNotFound) || status.Code(err) == codes.NotFound {
				httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not found", err))
				return
			}
			if err != nil {
				httpx.Error(w, httpx.NewError(http.StatusInternalServerError, "[JWT] Failed to resolve role", err))
				return
			}

			role = strings.ToLower(strings.TrimSpace(role))
			if _, ok := mp[role]; !ok {
				httpx.Error(w, httpx.NewError(http.StatusForbidden, "[JWT] Forbidden: Access denied", nil))
				return
			}

			next(w, r)
		}
	}
}
