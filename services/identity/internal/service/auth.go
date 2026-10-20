package service

import (
	"context"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/token"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func (s *service) Refresh(ctx context.Context, refreshToken string) (string, string, *httpx.HTTPError) {
	userID, err := s.jwtManager.ValidateRefresh(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, token.ErrInvalidToken) {
			return "", "", httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] Invalid refresh token", err)
		}
		if errors.Is(err, token.ErrTokenExpired) {
			return "", "", httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] Refresh token expired", err)
		}
		return "", "", httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to refresh token", err)
	}

	accessToken, newRefreshToken, err := s.jwtManager.Generate(ctx, userID)
	if err != nil {
		return "", "", httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to refresh token", err)
	}
	return accessToken, newRefreshToken, nil
}

func (s *service) Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, *httpx.HTTPError) {
	firebaseUser, err := s.firebaseClient.VerifyIDToken(ctx, body.IdToken)
	if err != nil {
		return nil, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] Invalid token", fmt.Errorf("%w: %w", ErrInvalidIdToken, err))
	}

	name := ""
	if firebaseUser.Name != nil {
		name = *firebaseUser.Name
	}
	if body.Name != nil {
		name = strings.TrimSpace(*body.Name)
	}

	avatar := ""
	if firebaseUser.Avatar != nil {
		avatar = *firebaseUser.Avatar
	}

	user, err := s.repo.UpsertUser(ctx, &model.User{
		Email:  firebaseUser.Email,
		Name:   name,
		Avatar: &avatar,
	})
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to login", err)
	}

	accessToken, refreshToken, err := s.jwtManager.Generate(ctx, strconv.Itoa(int(user.ID)))
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to login", err)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
