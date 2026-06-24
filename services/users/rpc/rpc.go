package rpc

import (
	"context"
	"strconv"

	usersClient "ego/api/gen/go/users"
	"ego/services/users/internal/model"
	"ego/services/users/internal/repository"
)

type server struct {
	usersClient.UnimplementedUserServiceServer
	repo *repository.Repository
}

func New(repo *repository.Repository) usersClient.UserServiceServer {
	return &server{
		repo: repo,
	}
}

func (s *server) UpsertUser(ctx context.Context, req *usersClient.UpsertUserRequest) (*usersClient.UpsertUserResponse, error) {
	userModel := &model.User{
		Email:  req.Email,
		Name:   req.Name,
		Avatar: &req.Avatar,
	}

	user, err := s.repo.UpsertUser(ctx, userModel)
	if err != nil {
		return nil, err
	}

	return &usersClient.UpsertUserResponse{
		Id: strconv.Itoa(int(user.ID)),
	}, nil
}
