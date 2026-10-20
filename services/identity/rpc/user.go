package rpc

import (
	"context"
	"errors"
	"strconv"

	identityClient "ego/api/gen/go/identity"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func (s *server) GetUserRole(ctx context.Context, req *identityClient.GetUserRoleRequest) (*identityClient.GetUserRoleResponse, error) {
	user, err := s.repo.GetByID(ctx, req.Id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, err
	}

	return &identityClient.GetUserRoleResponse{
		Role: string(user.Role),
	}, nil
}

func (s *server) GetUsersInfo(ctx context.Context, req *identityClient.GetUsersInfoRequest) (*identityClient.GetUsersInfoResponse, error) {
	users, err := s.repo.GetByIDs(ctx, req.Ids)
	if err != nil {
		return nil, err
	}

	res := &identityClient.GetUsersInfoResponse{
		Users: make([]*identityClient.UserInfo, len(users)),
	}
	for i, user := range users {
		avatar := ""
		if user.Avatar != nil {
			avatar = *user.Avatar
		}
		res.Users[i] = &identityClient.UserInfo{
			Id:     strconv.Itoa(int(user.ID)),
			Email:  user.Email,
			Name:   user.Name,
			Avatar: avatar,
		}
	}

	return res, nil
}

func (s *server) SearchUserIDsByName(ctx context.Context, req *identityClient.SearchUserIDsByNameRequest) (*identityClient.SearchUserIDsByNameResponse, error) {
	ids, err := s.repo.SearchUserIDsByName(ctx, req.Query)
	if err != nil {
		return nil, err
	}
	return &identityClient.SearchUserIDsByNameResponse{Ids: ids}, nil
}
