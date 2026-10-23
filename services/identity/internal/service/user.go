package service

import (
	"context"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

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

func (s *service) GetUsersInfo(ctx context.Context, userIDs []string) ([]*model.User, error) {
	return s.repo.GetByIDs(ctx, userIDs)
}

func (s *service) SearchUserIDsByName(ctx context.Context, query string) ([]string, error) {
	return s.repo.SearchUserIDsByName(ctx, query)
}

func (s *service) GetMe(ctx context.Context, userID string) (*dto.User, *httpx.HTTPError) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get user")
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
		Role:   string(user.Role),
	}, nil
}

func (s *service) UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, *httpx.HTTPError) {
	updates := map[string]any{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Avatar != nil {
		// The avatar must be a plain file name inside the avatars folder.
		if fileName := *body.Avatar; fileName == "" || fileName == "." || fileName == ".." || strings.ContainsAny(fileName, `/\`) {
			return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid avatar", nil)
		}
		resp, err := s.storageClient.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
			Folders:  []string{"avatars"},
			FileName: *body.Avatar,
		})
		if err != nil {
			return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get avatar URL", err)
		}
		updates["avatar"] = resp.Url
	}

	user, err := s.repo.UpdateMe(ctx, userID, updates)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update user")
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
		Role:   string(user.Role),
	}, nil
}

func (s *service) GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.User, int, *httpx.HTTPError) {
	users, total, err := s.repo.GetList(ctx, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get users", err)
	}

	userDTOs := make([]*dto.User, len(users))
	for i, user := range users {
		userDTOs[i] = &dto.User{
			ID:     user.ID,
			Email:  user.Email,
			Name:   user.Name,
			Avatar: user.Avatar,
			Role:   string(user.Role),
		}
	}

	return userDTOs, httpx.ToPageCounts(total, query.PageSize), nil
}
