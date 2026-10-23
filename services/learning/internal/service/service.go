package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	identityClient "ego/api/gen/go/identity"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/platform/httpx"
	learningConfig "ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"google.golang.org/grpc"
)

// ErrClassroomNotFound là lỗi nội bộ (rpc) khi không có lớp.
var ErrClassroomNotFound = errors.New("classroom not found")

// errClassroomServiceFailed là message lỗi 500 mặc định của các method HTTP phía classrooms (giữ nguyên từ HTTPService cũ).
const errClassroomServiceFailed = "[ERROR] Classroom service failed"

var (
	ErrClassroomNotActive     = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom is not active", nil)
	ErrClassroomForbidden     = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Classroom access denied", nil)
	ErrClassroomCapacityFull  = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom capacity is full", nil)
	ErrTeacherCannotJoinClass = httpx.NewError(http.StatusBadRequest, "[ERROR] Teacher cannot join own classroom as student", nil)
	ErrScheduleInvalidTime    = httpx.NewError(http.StatusBadRequest, "[ERROR] Schedule end time must be after start time", nil)
	ErrCalendarInvalidRange   = httpx.NewError(http.StatusBadRequest, "[ERROR] Calendar range must be valid and no longer than 366 days", nil)
	ErrRoomLive               = httpx.NewError(http.StatusConflict, "[CONFLICT] The room is currently live", nil)
	ErrRoomNotLive            = httpx.NewError(http.StatusBadRequest, "[ERROR] This session has not started yet", nil)
	ErrRoomEnded              = httpx.NewError(http.StatusBadRequest, "[ERROR] This session has ended", nil)
	ErrRoomTooEarly           = httpx.NewError(http.StatusBadRequest, "[ERROR] The room can only be started up to 30 minutes before the scheduled time", nil)
	ErrMediaUnavailable       = httpx.NewError(http.StatusServiceUnavailable, "[UNAVAILABLE] The media server is temporarily unavailable", nil)
)

// Lỗi phía exams/attempt (giữ nguyên status + message của exams cũ).
var (
	ErrAttemptNotActive            = httpx.NewError(http.StatusBadRequest, "[ERROR] Attempt is not active", nil)
	ErrAttemptExpired              = httpx.NewError(http.StatusBadRequest, "[ERROR] Attempt has expired", nil)
	ErrAttemptAlreadySubmitted     = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt has already been submitted", nil)
	ErrAttemptCancelled            = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt has been cancelled", nil)
	ErrAssignmentAttemptCancel     = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom assignment attempts cannot be cancelled; submit the attempt instead", nil)
	ErrQuestionNotBelongToAttempt  = httpx.NewError(http.StatusBadRequest, "[ERROR] Question does not belong to attempt", nil)
	ErrOptionNotBelongToQuestion   = httpx.NewError(http.StatusBadRequest, "[ERROR] Option does not belong to question", nil)
	ErrAttemptHistoryNotFound      = httpx.NewError(http.StatusNotFound, "[ERROR] Attempt history not found", nil)
	ErrContextAttemptAlreadyExists = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt already exists for this context", nil)
	ErrActiveAttemptExists         = httpx.NewError(http.StatusConflict, "[CONFLICT] Active attempt already exists", nil)
	ErrUnsupportedExamType         = httpx.NewError(http.StatusBadRequest, "[ERROR] Unsupported exam type", nil)
)

type Service interface {
	// Exam
	GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, *httpx.HTTPError)
	GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, *httpx.HTTPError)

	// Attempt
	CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int, *httpx.HTTPError)
	GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, *httpx.HTTPError)
	CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error)

	// Answer
	UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error)

	// Classroom
	CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError)
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError)
	GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, *httpx.HTTPError)
	GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError)
	GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, *httpx.HTTPError)
	UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, *httpx.HTTPError)
	ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, *httpx.HTTPError)
	GetClassroomPostAccess(ctx context.Context, classroomID uint, userID string) (*model.Classroom, bool, error)

	// Member
	RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, *httpx.HTTPError)
	RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) *httpx.HTTPError

	// Schedule
	CreateSchedule(ctx context.Context, teacherID string, classroomID uint, req dto.CreateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError)
	GetCalendar(ctx context.Context, userID string, classroomID uint, query dto.CalendarQuery) ([]*dto.CalendarResponse, *httpx.HTTPError)
	UpdateSchedule(ctx context.Context, teacherID string, scheduleID uint, req dto.UpdateScheduleRequest) (*dto.CalendarResponse, *httpx.HTTPError)
	DeleteSchedule(ctx context.Context, teacherID string, scheduleID uint) *httpx.HTTPError

	// Room
	GetRoom(ctx context.Context, userID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError)
	GetRoomActivity(ctx context.Context, userID string, classroomID uint, roomID uint) (*dto.RoomActivityResponse, *httpx.HTTPError)
	StartRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError)
	JoinRoom(ctx context.Context, userID string, roomID uint) (*dto.JoinRoomResponse, *httpx.HTTPError)
	EndRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, *httpx.HTTPError)
	GetRoomAttendance(ctx context.Context, userID string, roomID uint) ([]*dto.AttendanceSessionResponse, *httpx.HTTPError)
	ProcessRoomLifecycle(ctx context.Context) (int, error)

	// Screen share
	RequestScreenShare(ctx context.Context, userID string, classroomID, roomID uint) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	ApproveScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	RejectScreenShare(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	StartScreenShare(ctx context.Context, userID string, classroomID, roomID uint) *httpx.HTTPError
	StopScreenShare(ctx context.Context, actorID string, classroomID, roomID uint, userID string) *httpx.HTTPError

	// Hands
	SetHandRaised(ctx context.Context, userID string, classroomID, roomID uint, raised bool) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	LowerHands(ctx context.Context, teacherID string, classroomID, roomID uint, userID *string) (*dto.LoweredHandsResponse, *httpx.HTTPError)

	// Moderation
	MuteParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	AllowParticipantMicrophone(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	DisableParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	AllowParticipantCamera(ctx context.Context, teacherID string, classroomID, roomID uint, userID string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)
	MuteAllParticipants(ctx context.Context, teacherID string, classroomID, roomID uint) (*dto.MutedParticipantsResponse, *httpx.HTTPError)
	RemoveParticipant(ctx context.Context, teacherID string, classroomID, roomID uint, userID string, reason *string) (*dto.RoomParticipantStateResponse, *httpx.HTTPError)

	// Webhook LiveKit
	HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error
	HandleParticipantLeft(ctx context.Context, roomName, userID string, leftAt time.Time) error
	HandleScreenSharePublished(ctx context.Context, roomName, userID string) error
	HandleScreenShareUnpublished(ctx context.Context, roomName, userID string) error

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
