package rpc

import (
	"context"
	"errors"
	"strconv"

	identityClient "ego/api/gen/go/identity"
	platformRpc "ego/platform/rpc"
	"ego/services/identity/internal/service"

	"google.golang.org/grpc/codes"
)

func (s *server) GetUserRole(ctx context.Context, req *identityClient.GetUserRoleRequest) (*identityClient.GetUserRoleResponse, error) {
	role, err := s.service.GetRole(ctx, req.Id)
	if errors.Is(err, service.ErrUserNotFound) {
		return nil, platformRpc.PublicError(codes.NotFound, "[ERROR] User not found")
	}
	if err != nil {
		return nil, err
	}

	return &identityClient.GetUserRoleResponse{
		Role: role,
	}, nil
}

func (s *server) GetUsersInfo(ctx context.Context, req *identityClient.GetUsersInfoRequest) (*identityClient.GetUsersInfoResponse, error) {
	users, err := s.service.GetUsersInfo(ctx, req.Ids)
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
	ids, err := s.service.SearchUserIDsByName(ctx, req.Query)
	if err != nil {
		return nil, err
	}
	return &identityClient.SearchUserIDsByNameResponse{Ids: ids}, nil
}
