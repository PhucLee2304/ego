package service

import (
	"context"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/firebase"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/token"
	"errors"
)

var ErrInvalidIdToken = errors.New("invalid id token")

type Service interface {
	Refresh(ctx context.Context, refreshToken string) (string, string, *httpx.HTTPError)
	Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, *httpx.HTTPError)
	GetMe(ctx context.Context, userID string) (*dto.User, *httpx.HTTPError)
	UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, *httpx.HTTPError)
	GetRole(ctx context.Context, userID string) (string, error)
	GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.User, int, *httpx.HTTPError)
}

type service struct {
	jwtManager     token.Manager
	firebaseClient firebase.Client
	repo           *repository.Repository
	storageClient  storageClient.StorageServiceClient
}

func New(
	jwtManager token.Manager,
	firebaseClient firebase.Client,
	repo *repository.Repository,
	storageClient storageClient.StorageServiceClient,
) Service {
	return &service{
		jwtManager:     jwtManager,
		firebaseClient: firebaseClient,
		repo:           repo,
		storageClient:  storageClient,
	}
}
