package service

import (
	"context"
	"ego/services/users/internal/dto"
	"ego/services/users/internal/repository"
)

type Service interface {
	GetMe(ctx context.Context, userID string) (*dto.User, error)
	UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, error)
	GetRole(ctx context.Context, userID string) (string, error)
	GetList(ctx context.Context) ([]*dto.User, error)
}

type service struct {
	userRepo *repository.UserRepository
}

func New(userRepo *repository.UserRepository) Service {
	return &service{userRepo: userRepo}
}

func (s *service) GetRole(ctx context.Context, userID string) (string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return string(user.Role), nil
}

func (s *service) GetMe(ctx context.Context, userID string) (*dto.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
	}, nil
}

func (s *service) UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if body.Name != nil {
		user.Name = *body.Name
	}
	if body.Avatar != nil {
		user.Avatar = body.Avatar
	}

	user, err = s.userRepo.UpdateMe(ctx, user)
	if err != nil {
		return nil, err
	}
	return &dto.User{
		ID:     user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Avatar: user.Avatar,
	}, nil
}

func (s *service) GetList(ctx context.Context) ([]*dto.User, error) {
	users, err := s.userRepo.GetList(ctx)
	if err != nil {
		return nil, err
	}

	userDTOs := make([]*dto.User, len(users))
	for i, user := range users {
		userDTOs[i] = &dto.User{
			ID:     user.ID,
			Email:  user.Email,
			Name:   user.Name,
			Avatar: user.Avatar,
		}
	}

	return userDTOs, nil
}
