package socket

import (
	"context"
	tokenClient "ego/api/gen/go/token"
	"errors"
	"net/http"
	"strings"
)

var ErrUnauthorized = errors.New("socket authentication failed")

func AuthenticateToken(ctx context.Context, r *http.Request, tokenServiceClient tokenClient.TokenServiceClient) (string, error) {
	token := TokenFromRequest(r)
	if token == "" {
		return "", ErrUnauthorized
	}

	resp, err := tokenServiceClient.ValidateToken(ctx, &tokenClient.ValidateTokenRequest{
		Token: token,
	})
	if err != nil || !resp.IsValid {
		return "", ErrUnauthorized
	}

	userID := strings.TrimSpace(resp.UserId)
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

	return ""
}
