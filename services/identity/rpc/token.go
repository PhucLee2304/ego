package rpc

import (
	"context"
	"errors"

	identityClient "ego/api/gen/go/identity"
	"ego/services/identity/internal/service"
)

func (s *server) ValidateToken(ctx context.Context, req *identityClient.ValidateTokenRequest) (*identityClient.ValidateTokenResponse, error) {
	userID, err := s.service.ValidateToken(ctx, req.Token)
	if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrUserBlocked) {
		return &identityClient.ValidateTokenResponse{IsValid: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return &identityClient.ValidateTokenResponse{
		IsValid: true,
		UserId:  userID,
	}, nil
}
