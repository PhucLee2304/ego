package service

import (
	"context"
	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
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
	ErrUserBlocked    = errors.New("user is blocked")
	ErrUserNotFound   = jwt.ErrUserNotFound
	ErrInvalidToken   = jwt.ErrInvalidToken
)

type Service interface {
	// Auth
	Refresh(ctx context.Context, refreshToken string) (string, string, *httpx.HTTPError)
	Login(ctx context.Context, body dto.LoginBody) (*dto.LoginResponse, *httpx.HTTPError)

	// User
	GetMe(ctx context.Context, userID string) (*dto.User, *httpx.HTTPError)
	UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, *httpx.HTTPError)
	GetList(ctx context.Context, query dto.UsersQuery) ([]*dto.User, int64, *httpx.HTTPError)
	SetBlocked(ctx context.Context, userID string, blocked bool) (*dto.User, *httpx.HTTPError)

	// Stats (admin)
	GetAdminStats(ctx context.Context, query dto.AdminStatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError)

	// RPC
	ValidateToken(ctx context.Context, tokenString string) (string, error)
	GetRole(ctx context.Context, userID string) (string, error)
	GetUsersInfo(ctx context.Context, userIDs []string) ([]*model.User, error)
	SearchUserIDsByName(ctx context.Context, query string) ([]string, error)

	// Worker
	ProcessUserBlockedOutbox(ctx context.Context, limit int) (int, error)
}

type IdentityProvider interface {
	VerifyIDToken(ctx context.Context, idToken string) (*firebase.FirebaseUser, error)
}

type StorageClient interface {
	GetPublicURL(ctx context.Context, in *storageClient.GetPublicURLRequest, opts ...grpc.CallOption) (*storageClient.GetPublicURLResponse, error)
}

type NotificationsClient interface {
	DeleteDeviceTokensByUserID(ctx context.Context, in *notificationsClient.DeleteDeviceTokensByUserIDRequest, opts ...grpc.CallOption) (*notificationsClient.DeleteDeviceTokensByUserIDResponse, error)
}
type LearningClient interface {
	RemoveUserFromLiveRooms(ctx context.Context, in *learningClient.RemoveUserFromLiveRoomsRequest, opts ...grpc.CallOption) (*learningClient.RemoveUserFromLiveRoomsResponse, error)
}

type service struct {
	jwtManager          token.Manager
	firebaseClient      IdentityProvider
	repo                repository.Repository
	storageClient       StorageClient
	notificationsClient NotificationsClient
	learningClient      LearningClient
}

func New(
	jwtManager token.Manager,
	firebaseClient IdentityProvider,
	repo repository.Repository,
	storageClient StorageClient,
	notificationsClient NotificationsClient,
	learningClient LearningClient,
) Service {
	return &service{
		jwtManager:          jwtManager,
		firebaseClient:      firebaseClient,
		repo:                repo,
		storageClient:       storageClient,
		notificationsClient: notificationsClient,
		learningClient:      learningClient,
	}
}
