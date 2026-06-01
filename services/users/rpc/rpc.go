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
	userRepo *repository.UserRepository
}

func New(userRepo *repository.UserRepository) usersClient.UserServiceServer {
	return &server{
		userRepo: userRepo,
	}
}

func (s *server) UpsertUser(ctx context.Context, req *usersClient.UpsertUserRequest) (*usersClient.UpsertUserResponse, error) {
	userModel := &model.User{
		Email:  req.Email,
		Name:   req.Name,
		Avatar: &req.Avatar,
	}

	user, err := s.userRepo.UpsertUser(ctx, userModel)
	if err != nil {
		return nil, err
	}

	return &usersClient.UpsertUserResponse{
		Id: strconv.Itoa(int(user.ID)),
	}, nil
}
