package service

import (
	"context"
	"net/http"

	identityClient "ego/api/gen/go/identity"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/httpx"
	learningConfig "ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/repository"

	"google.golang.org/grpc"
)

// errClassroomServiceFailed là message lỗi 500 mặc định của các method HTTP phía classrooms (giữ nguyên từ HTTPService cũ).
const errClassroomServiceFailed = "[ERROR] Classroom service failed"

var (
	ErrClassroomNotActive     = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom is not active", nil)
	ErrClassroomForbidden     = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Classroom access denied", nil)
	ErrClassroomCapacityFull  = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom capacity is full", nil)
	ErrTeacherCannotJoinClass = httpx.NewError(http.StatusBadRequest, "[ERROR] Teacher cannot join own classroom as student", nil)
)

type Service interface {
	// Classroom
	CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError)
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError)
	GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError)
	GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError)
	GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, *httpx.HTTPError)
	UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, *httpx.HTTPError)
	ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError)

	// Member
	RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) *httpx.HTTPError

	// Notification outbox
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)
}

// NotificationClient là interface hẹp tới notifications service, để test bằng fake.
type NotificationClient interface {
	CreateNotifications(ctx context.Context, in *notificationsClient.CreateNotificationsRequest, opts ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error)
}

// IdentityClient là interface hẹp tới identity service: chỉ các method classrooms dùng.
type IdentityClient interface {
	GetUsersInfo(ctx context.Context, in *identityClient.GetUsersInfoRequest, opts ...grpc.CallOption) (*identityClient.GetUsersInfoResponse, error)
	SearchUserIDsByName(ctx context.Context, in *identityClient.SearchUserIDsByNameRequest, opts ...grpc.CallOption) (*identityClient.SearchUserIDsByNameResponse, error)
}

// EventPublisher đẩy payload realtime tới mọi kết nối WebSocket của một user.
type EventPublisher func(userID string, payload any)

type service struct {
	repo               repository.Repository
	cfg                *learningConfig.AppConfig
	notificationClient NotificationClient
	identityClient     IdentityClient
	mediaClient        media.Client
	roomPublisher      EventPublisher // hub /ws/classrooms
	attemptPublisher   EventPublisher // hub /ws/exams
}

func New(
	repo repository.Repository,
	cfg *learningConfig.AppConfig,
	notificationClient NotificationClient,
	identityClient IdentityClient,
	mediaClient media.Client,
	roomPublisher EventPublisher,
	attemptPublisher EventPublisher,
) Service {
	return &service{
		repo:               repo,
		cfg:                cfg,
		notificationClient: notificationClient,
		identityClient:     identityClient,
		mediaClient:        mediaClient,
		roomPublisher:      roomPublisher,
		attemptPublisher:   attemptPublisher,
	}
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}
