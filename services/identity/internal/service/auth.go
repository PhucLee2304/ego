package service

import (
	"context"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/token"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

func (s *service) ValidateToken(ctx context.Context, tokenString string) (string, error) {
	return s.jwtManager.Validate(ctx, tokenString)
}

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

	user, err := s.repo.GetByID(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", "", httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] Invalid refresh token", err)
	}
	if err != nil {
		return "", "", httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to refresh token", err)
	}
	if user.BlockedAt != nil {
		return "", "", httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Account blocked", ErrUserBlocked)
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
	firebaseUID := strings.TrimSpace(firebaseUser.UID)
	email := strings.TrimSpace(firebaseUser.Email)
	if firebaseUID == "" || email == "" {
		return nil, httpx.NewError(
			http.StatusUnauthorized,
			"[UNAUTHORIZED] Token is missing required identity claims",
			fmt.Errorf("%w: missing Firebase UID or email", ErrInvalidIdToken),
		)
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

	var accessToken, refreshToken string
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		user, err := repo.UpsertUser(ctx, &model.User{
			FirebaseUID: firebaseUID,
			Email:       email,
			Name:        name,
			Avatar:      &avatar,
		})
		if err != nil {
			return err
		}
		// Rollback cả lần cập nhật email: tài khoản bị khoá không được ghi gì khi đăng nhập.
		if user.BlockedAt != nil {
			return ErrUserBlocked
		}

		accessToken, refreshToken, err = s.jwtManager.Generate(ctx, strconv.Itoa(int(user.ID)))
		return err
	})
	if errors.Is(err, ErrUserBlocked) {
		return nil, httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Account blocked", err)
	}
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to login", err)
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
