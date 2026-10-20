package socket

import (
	"context"
	identityClient "ego/api/gen/go/identity"
	"errors"
	"net/http"
	"strings"
)

var ErrUnauthorized = errors.New("socket authentication failed")

func AuthenticateToken(ctx context.Context, r *http.Request, identityServiceClient identityClient.IdentityServiceClient) (string, error) {
	token := TokenFromRequest(r)
	if token == "" {
		return "", ErrUnauthorized
	}

	tokenResp, err := identityServiceClient.ValidateToken(ctx, &identityClient.ValidateTokenRequest{
		Token: token,
	})
	if err != nil {
		// Identity could not judge the token; keep the gRPC error so the caller
		// does not report a valid token as unauthorized.
		return "", err
	}
	if !tokenResp.IsValid {
		return "", ErrUnauthorized
	}

	userID := strings.TrimSpace(tokenResp.UserId)
	if userID == "" {
		return "", ErrUnauthorized
	}

	return userID, nil
}

func TokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	// Browser WebSocket APIs cannot set an Authorization header during the
	// handshake, so web clients use this fallback over WSS.
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}
