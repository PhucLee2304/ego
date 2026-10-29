package service

import (
	"context"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/firebase"
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/token"
	"errors"

	"google.golang.org/grpc"
)

var (
	ErrInvalidIdToken = errors.New("invalid id token")
	// ErrUserNotFound is the role middleware's sentinel so GetRole can serve as
	// a jwt.RoleResolver without leaking GORM errors.
	ErrUserNotFound = jwt.ErrUserNotFound
)

type Service interface {
	// Auth
	Refresh(ctx context.Context, refreshToken string) (string, string, *httpx.HTTPError)
	Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, *httpx.HTTPError)

	// User
	GetMe(ctx context.Context, userID string) (*dto.User, *httpx.HTTPError)
	UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, *httpx.HTTPError)
	GetRole(ctx context.Context, userID string) (string, error)
	GetList(ctx context.Context, query httpx.PaginationQuery) ([]*dto.User, int64, *httpx.HTTPError)

	// RPC
	ValidateToken(ctx context.Context, tokenString string) (string, error)
	GetUsersInfo(ctx context.Context, userIDs []string) ([]*model.User, error)
	SearchUserIDsByName(ctx context.Context, query string) ([]string, error)
}

type IdentityProvider interface {
	VerifyIDToken(ctx context.Context, idToken string) (*firebase.FirebaseUser, error)
}

type StorageClient interface {
	GetPublicURL(ctx context.Context, in *storageClient.GetPublicURLRequest, opts ...grpc.CallOption) (*storageClient.GetPublicURLResponse, error)
}

type service struct {
	jwtManager     token.Manager
	firebaseClient IdentityProvider
	repo           repository.Repository
	storageClient  StorageClient
}

func New(
	jwtManager token.Manager,
	firebaseClient IdentityProvider,
	repo repository.Repository,
	storageClient StorageClient,
) Service {
	return &service{
		jwtManager:     jwtManager,
		firebaseClient: firebaseClient,
		repo:           repo,
		storageClient:  storageClient,
	}
}
