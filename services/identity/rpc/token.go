package rpc

import (
	"context"

	identityClient "ego/api/gen/go/identity"
)

func (s *server) ValidateToken(ctx context.Context, req *identityClient.ValidateTokenRequest) (*identityClient.ValidateTokenResponse, error) {
	userID, err := s.service.ValidateToken(ctx, req.Token)
	if err != nil {
		// An invalid or expired token is a normal answer, not an RPC failure.
		return &identityClient.ValidateTokenResponse{IsValid: false}, nil
	}
	return &identityClient.ValidateTokenResponse{
		IsValid: true,
		UserId:  userID,
	}, nil
}
