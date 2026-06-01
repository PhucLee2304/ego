package service

import (
	"context"
	"errors"

	usersClient "ego/api/gen/go/users"
	"ego/platform/firebase"
	"ego/platform/jwt"
	"ego/services/auth/internal/dto"
)

var (
	ErrInvalidIdToken = errors.New("invalid id token")
)

type Service interface {
	Refresh(ctx context.Context, refreshToken string) (string, string, error)
	Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, error)
}

type service struct {
	jwtManager     jwt.Manager
	firebaseClient firebase.Client
	usersClient    usersClient.UserServiceClient
}

func New(
	jwtManager jwt.Manager,
	firebaseClient firebase.Client,
	usersClient usersClient.UserServiceClient,
) Service {
	return &service{
		jwtManager:     jwtManager,
		firebaseClient: firebaseClient,
		usersClient:    usersClient,
	}
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (string, string, error) {
	userID, err := s.jwtManager.Validate(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}
	return s.jwtManager.Generate(ctx, userID)
}

func (s *service) Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, error) {
	firebaseUser, err := s.firebaseClient.VerifyIDToken(ctx, body.IdToken)
	if err != nil {
		return nil, ErrInvalidIdToken
	}

	name := ""
	if firebaseUser.Name != nil {
		name = *firebaseUser.Name
	}
	if body.Name != nil && *body.Name != "" {
		name = *body.Name
	}

	avatar := ""
	if firebaseUser.Avatar != nil {
		avatar = *firebaseUser.Avatar
	}

	user, err := s.usersClient.UpsertUser(ctx, &usersClient.UpsertUserRequest{
		Email:  firebaseUser.Email,
		Name:   name,
		Avatar: avatar,
	})
	if err != nil {
		return nil, err
	}

	accessToken, refreshToken, err := s.jwtManager.Generate(ctx, user.Id)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
