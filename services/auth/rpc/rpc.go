package rpc

import (
	"context"

	"ego/api/gen/go/token"
	"ego/platform/jwt"
)

type server struct {
	token.UnimplementedTokenServiceServer
	jwtManager jwt.Manager
}

func New(jwtManager jwt.Manager) token.TokenServiceServer {
	return &server{
		jwtManager: jwtManager,
	}
}

func (s *server) GenerateToken(ctx context.Context, req *token.GenerateTokenRequest) (*token.GenerateTokenResponse, error) {
	accessToken, refreshToken, err := s.jwtManager.Generate(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &token.GenerateTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *server) ValidateToken(ctx context.Context, req *token.ValidateTokenRequest) (*token.ValidateTokenResponse, error) {
	userId, err := s.jwtManager.Validate(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	return &token.ValidateTokenResponse{
		UserId: userId,
	}, nil
}
