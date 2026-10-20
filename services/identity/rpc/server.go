package rpc

import (
	identityClient "ego/api/gen/go/identity"
	"ego/services/identity/internal/service"
)

type server struct {
	identityClient.UnimplementedIdentityServiceServer
	service service.Service
}

func New(svc service.Service) identityClient.IdentityServiceServer {
	return &server{service: svc}
}
