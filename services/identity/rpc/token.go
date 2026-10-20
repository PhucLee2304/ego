package rpc

import (
	"context"

	identityClient "ego/api/gen/go/identity"
)

func (s *server) ValidateToken(ctx context.Context, req *identityClient.ValidateTokenRequest) (*identityClient.ValidateTokenResponse, error) {
	userId, err := s.jwtManager.Validate(ctx, req.Token)
	if err != nil {
		// An invalid or expired token is a normal answer, not an RPC failure.
		return &identityClient.ValidateTokenResponse{IsValid: false}, nil
	}
	return &identityClient.ValidateTokenResponse{
		IsValid: true,
		UserId:  userId,
	}, nil
}
