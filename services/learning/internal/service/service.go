package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	identityClient "ego/api/gen/go/identity"
	notificationsClient "ego/api/gen/go/notifications"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/httpx"
	learningConfig "ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/media"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

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

	ErrAssignmentInvalidTime          = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment due time must be after open time", nil)
	ErrAssignmentDuration             = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment duration must not exceed the open time window", nil)
	ErrAssignmentExpired              = httpx.NewError(http.StatusBadRequest, "[ERROR] An expired assignment cannot be activated", nil)
	ErrAssignmentAlreadyExists        = httpx.NewError(http.StatusConflict, "[CONFLICT] This exam is already assigned to the classroom", nil)
	ErrAssignmentScheduleConfirmation = httpx.NewError(http.StatusConflict, "[CONFIRMATION_REQUIRED] Active attempts may be affected by this schedule change", nil)
	ErrAssignmentContentConfirmation  = httpx.NewError(http.StatusConflict, "[CONFIRMATION_REQUIRED] This exam already has attempts; confirm to submit active attempts and pause all related assignments", nil)
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
	ErrAttemptPagedFlow            = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt uses paged flow", nil)
	ErrAttemptPageNotFound         = httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Attempt page not found", nil)
	ErrAttemptPageLocked           = httpx.NewError(http.StatusConflict, "[CONFLICT] PAGE_LOCKED: Attempt page is locked", nil)
	ErrUnsupportedExamType         = httpx.NewError(http.StatusBadRequest, "[ERROR] Unsupported exam type", nil)
)

// Lỗi giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md).
var (
	ErrIntegrityConsentRequired   = httpx.NewError(http.StatusBadRequest, "[ERROR] Integrity consent is required for a proctored assignment", nil)
	ErrIntegrityConsentOutdated   = httpx.NewError(http.StatusConflict, "[CONFLICT] Consent version outdated", nil)
	ErrAttemptNotProctored        = httpx.NewError(http.StatusBadRequest, "[ERROR] Attempt is not proctored", nil)
	ErrAttemptNotTerminated       = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt is not suspended", nil)
	ErrIntegrityEventNotFound     = httpx.NewError(http.StatusNotFound, "[NOT_FOUND] Integrity event not found", nil)
	ErrIntegrityEventNotViolation = httpx.NewError(http.StatusBadRequest, "[ERROR] Only violations can be reviewed", nil)
	ErrIntegrityReviewConflict    = httpx.NewError(http.StatusConflict, "[CONFLICT] Integrity event was already reviewed with another action", nil)
	ErrIntegrityLevelInvalid      = httpx.NewError(http.StatusBadRequest, "[ERROR] Level must be LOG, WARNING or VIOLATION", nil)
)

// Lỗi kiểm tra attempt của assignment (thay ValidateAssignmentAttempt /
// CreateAssignmentSubmission / UpdateAssignmentSubmissionResult): giữ đúng status
// + message mà exams cũ trả sau toStatusError + httpx.FromError.
var (
	ErrAssignmentNotFound     = httpx.NewError(http.StatusNotFound, "[ERROR] Assignment not found", nil)
	ErrAssignmentExamMismatch = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment does not belong to exam", nil)
	ErrAssignmentNotOpen      = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment is not open yet", nil)
	ErrAssignmentClosed       = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment is closed", nil)
	ErrAssignmentNotActive    = httpx.NewError(http.StatusBadRequest, "[ERROR] Assignment is not active", nil)
	ErrStudentNotApproved     = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] Student is not approved in classroom", nil)
	ErrInvalidSubmissionScore = httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid submission score", nil)
)

type Service interface {
	// Exam
	GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int64, *httpx.HTTPError)
	GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, *httpx.HTTPError)

	// Attempt
	CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int64, *httpx.HTTPError)
	GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, *httpx.HTTPError)
	CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error)

	// Attempt page (luồng theo trang)
	GetAttemptNavigator(ctx context.Context, userID string, attemptID uint) (*dto.AttemptNavigatorResponse, *httpx.HTTPError)
	GetAttemptPage(ctx context.Context, userID string, attemptID uint, index int) (*dto.AttemptPageResponse, *httpx.HTTPError)
	AdvanceListening(ctx context.Context, userID string, attemptID uint, page int) (*dto.ListeningStateResponse, error)

	// Answer
	UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error)

	// Classroom
	CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, *httpx.HTTPError)
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int64, *httpx.HTTPError)
	GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int64, *httpx.HTTPError)
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

	// Assignment
	CreateAssignments(ctx context.Context, teacherID string, req dto.CreateAssignmentsRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError)
	AssignExistingExam(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.AssignExistingExamRequest) (*dto.CreatedAssignmentsResponse, *httpx.HTTPError)
	GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int64, *httpx.HTTPError)
	GetAssignment(ctx context.Context, userID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError)
	UpdateAssignmentSchedule(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, *httpx.HTTPError)
	SetAssignmentActive(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, active bool) (*dto.AssignmentResponse, *httpx.HTTPError)

	// Assignment edit
	GetEditableAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.EditableAssignmentResponse, *httpx.HTTPError)
	UpdateAssignmentContent(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.EditableAssignmentResponse, *httpx.HTTPError)
	ResetAssignmentAttempt(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, studentID string) *httpx.HTTPError
	GetAssignmentAudits(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) ([]*dto.ExamAuditResponse, *httpx.HTTPError)

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

	// Gradebook
	GetGradebook(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, *httpx.HTTPError)
	GetClassGradebook(ctx context.Context, teacherID string, classroomID uint, query dto.GradebookRangeQuery) (*dto.ClassGradebookResponse, *httpx.HTTPError)
	GetStudentProgress(ctx context.Context, userID string, classroomID uint, studentID string) (*dto.StudentProgressResponse, *httpx.HTTPError)
	GetAssignmentAnalytics(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentAnalyticsResponse, *httpx.HTTPError)
	ReconcileAssignmentSubmissions(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (int, *httpx.HTTPError)

	// Notification outbox
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)

	// Attempt integrity (giám sát; WS phía học sinh, REST phía giáo viên)
	ClaimAttemptSession(ctx context.Context, userID, connectionID string, payload socket.AttemptSessionClaimPayload) (*dto.AttemptSessionClaimResponse, error)
	RecordIntegrityEvent(ctx context.Context, userID, connectionID string, payload socket.AttemptIntegrityEventPayload) (*dto.IntegrityEventAckResponse, error)
	ReleaseAttemptConnection(ctx context.Context, connectionID string)
	GetIntegrityEvents(ctx context.Context, teacherID string, classroomID, assignmentID uint, query dto.IntegrityEventsQuery) ([]*dto.AttemptIntegrityEventResponse, int64, *httpx.HTTPError)
	ReviewIntegrityEvent(ctx context.Context, teacherID string, classroomID, assignmentID uint, eventID string, req dto.ReviewIntegrityEventRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError)
	BanAttempt(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string, req dto.BanAttemptRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError)
	ReleaseAttemptSession(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string) (*dto.AttemptIntegrityResponse, *httpx.HTTPError)
	ReinstateAttempt(ctx context.Context, teacherID string, classroomID, assignmentID uint, studentID string, req dto.ReinstateAttemptRequest) (*dto.AttemptIntegrityResponse, *httpx.HTTPError)
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

// StorageClient là interface hẹp tới storage service: chỉ dùng để đổi tên file ảnh lớp thành URL công khai.
type StorageClient interface {
	GetPublicURL(ctx context.Context, in *storageClient.GetPublicURLRequest, opts ...grpc.CallOption) (*storageClient.GetPublicURLResponse, error)
}

// EventPublisher đẩy payload realtime tới mọi kết nối WebSocket của một user.
type EventPublisher func(userID string, payload any)

type service struct {
	repo               repository.Repository
	cfg                *learningConfig.AppConfig
	notificationClient NotificationClient
	identityClient     IdentityClient
	storageClient      StorageClient
	mediaClient        media.Client
	roomPublisher      EventPublisher // hub /ws/classrooms
	attemptPublisher   EventPublisher // hub /ws/exams
	integrity          *integrityRegistry
	now                func() time.Time
}

func New(
	repo repository.Repository,
	cfg *learningConfig.AppConfig,
	notificationClient NotificationClient,
	identityClient IdentityClient,
	storageClient StorageClient,
	mediaClient media.Client,
	roomPublisher EventPublisher,
	attemptPublisher EventPublisher,
) Service {
	return &service{
		repo:               repo,
		cfg:                cfg,
		notificationClient: notificationClient,
		identityClient:     identityClient,
		storageClient:      storageClient,
		mediaClient:        mediaClient,
		roomPublisher:      roomPublisher,
		attemptPublisher:   attemptPublisher,
		integrity:          newIntegrityRegistry(),
		now:                time.Now,
	}
}

// SetClock thay đồng hồ của service; chỉ dùng trong test (luồng nghe phụ thuộc thời gian).
func SetClock(svc Service, now func() time.Time) {
	if s, ok := svc.(*service); ok {
		s.now = now
	}
}

func (s *service) clock() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}

// withRepo trả bản sao service dùng repo của transaction đang mở, để gọi logic
// của domain khác trong cùng một transaction (CONVENTIONS.md mục 6).
func (s *service) withRepo(repo repository.Repository) *service {
	copy := *s
	copy.repo = repo
	return &copy
}

// pendingEvent là một push WebSocket đang chờ transaction ngoài commit.
type pendingEvent struct {
	room    bool // true: hub /ws/classrooms (GRADEBOOK_UPDATED), false: hub /ws/exams
	userID  string
	payload any
}

// withRepoDeferringEvents như withRepo, nhưng gom push ATTEMPT_SUBMITTING/SUBMITTED
// và GRADEBOOK_UPDATED vào events thay vì gửi ngay: chỉ gửi bằng publishPendingEvents
// sau khi transaction ngoài commit, để client không nhận "đã nộp" hay điểm của một
// transaction bị rollback.
func (s *service) withRepoDeferringEvents(repo repository.Repository, events *[]pendingEvent) *service {
	copy := s.withRepo(repo)
	copy.attemptPublisher = func(userID string, payload any) {
		*events = append(*events, pendingEvent{userID: userID, payload: payload})
	}
	copy.roomPublisher = func(userID string, payload any) {
		*events = append(*events, pendingEvent{room: true, userID: userID, payload: payload})
	}
	return copy
}

func (s *service) publishPendingEvents(events []pendingEvent) {
	for _, event := range events {
		publisher := s.attemptPublisher
		if event.room {
			publisher = s.roomPublisher
		}
		if publisher != nil {
			publisher(event.userID, event.payload)
		}
	}
}
