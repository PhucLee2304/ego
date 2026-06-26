package service

import (
	"context"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/httpx"
	"ego/services/users/internal/dto"
	"ego/services/users/internal/repository"
)

type Service interface {
	GetMe(ctx context.Context, userID string) (*dto.User, error)
	UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, error)
	GetRole(ctx context.Context, userID string) (string, error)
	GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.User, int, error)
}

type service struct {
	repo          *repository.Repository
	storageClient storageClient.StorageServiceClient
}

func New(repo *repository.Repository, storageClient storageClient.StorageServiceClient) Service {
	return &service{repo: repo, storageClient: storageClient}
}

func (s *service) GetRole(ctx context.Context, userID string) (string, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return string(user.Role), nil
}

func (s *service) GetMe(ctx context.Context, userID string) (*dto.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
		Role:   string(user.Role),
	}, nil
}

func (s *service) UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, error) {
	updates := map[string]any{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Avatar != nil {
		resp, err := s.storageClient.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
			Folders:  []string{"avatars"},
			FileName: *body.Avatar,
		})
		if err != nil {
			return nil, err
		}
		updates["avatar"] = resp.Url
	}

	user, err := s.repo.UpdateMe(ctx, userID, updates)
	if err != nil {
		return nil, err
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
		Role:   string(user.Role),
	}, nil
}

func (s *service) GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.User, int, error) {
	users, total, err := s.repo.GetList(ctx, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, err
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

	pageCounts := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if pageCounts == 0 {
		pageCounts = 1
	}

	return userDTOs, pageCounts, nil
}
