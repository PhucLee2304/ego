package service

import (
	"context"
	"ego/services/identity/internal/model"
	"errors"

	"gorm.io/gorm"
)

// **Reviewed**
func (s *service) ValidateToken(ctx context.Context, tokenString string) (string, error) {
	return s.jwtManager.Validate(ctx, tokenString)
}

// **Reviewed**
func (s *service) GetRole(ctx context.Context, userID string) (string, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrUserNotFound
	}
	if err != nil {
		return "", err
	}
	return string(user.Role), nil
}

// **Reviewed**
func (s *service) GetUsersInfo(ctx context.Context, userIDs []string) ([]*model.User, error) {
	return s.repo.GetByIDs(ctx, userIDs)
}

// **Reviewed**
func (s *service) SearchUserIDsByName(ctx context.Context, query string) ([]string, error) {
	return s.repo.SearchUserIDsByName(ctx, query)
}
