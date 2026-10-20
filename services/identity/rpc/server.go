package rpc

import (
	identityClient "ego/api/gen/go/identity"
	"ego/services/identity/internal/repository"
	tokenManager "ego/services/identity/internal/token"
)

type server struct {
	identityClient.UnimplementedIdentityServiceServer
	jwtManager tokenManager.Manager
	repo       *repository.Repository
}

func New(jwtManager tokenManager.Manager, repo *repository.Repository) identityClient.IdentityServiceServer {
	return &server{
		jwtManager: jwtManager,
		repo:       repo,
	}
}
