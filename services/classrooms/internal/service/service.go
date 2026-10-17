package service

import (
	"context"
	"crypto/rand"
	examsClient "ego/api/gen/go/exams"
	notificationsClient "ego/api/gen/go/notifications"
	usersClient "ego/api/gen/go/users"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/platform/valuex"
	classroomsConfig "ego/services/classrooms/config"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/media"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Service interface {
	CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, error)
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error)
	GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error)
	GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, error)
	GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, error)
	UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error)
	ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, error)
	RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, error)
	GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, error)
	ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error)
	RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error)
	RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) error
	CreateAssignments(ctx context.Context, teacherID string, req dto.CreateAssignmentsRequest) (*dto.CreatedAssignmentsResponse, error)
	AssignExistingExam(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.AssignExistingExamRequest) (*dto.CreatedAssignmentsResponse, error)
	UpdateAssignmentSchedule(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, error)
	GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int, error)
	GetAssignment(ctx context.Context, userID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, error)
	SetAssignmentActive(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, active bool) (*dto.AssignmentResponse, error)
	GetGradebook(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, error)
	GetClassGradebook(ctx context.Context, teacherID string, classroomID uint, query dto.GradebookRangeQuery) (*dto.ClassGradebookResponse, error)
	GetStudentProgress(ctx context.Context, userID string, classroomID uint, studentID string) (*dto.StudentProgressResponse, error)
	GetAssignmentAnalytics(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentAnalyticsResponse, error)
	GetEditableAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.EditableAssignmentResponse, error)
	UpdateAssignmentContent(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.EditableAssignmentResponse, error)
	ResetAssignmentAttempt(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, studentID string) error
	ReconcileAssignmentSubmissions(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (int, error)
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)
	SetNotificationClient(notificationsClient.NotificationServiceClient)
	GetAssignmentAudits(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) ([]*dto.ExamAuditResponse, error)
	CreateSchedule(ctx context.Context, teacherID string, classroomID uint, req dto.CreateScheduleRequest) (*dto.CalendarResponse, error)
	GetCalendar(ctx context.Context, userID string, classroomID uint, query dto.CalendarQuery) ([]*dto.CalendarResponse, error)
	UpdateSchedule(ctx context.Context, teacherID string, scheduleID uint, req dto.UpdateScheduleRequest) (*dto.CalendarResponse, error)
	DeleteSchedule(ctx context.Context, teacherID string, scheduleID uint) error
	GetRoom(ctx context.Context, userID string, roomID uint) (*dto.RoomResponse, error)
	GetRoomActivity(ctx context.Context, userID string, classroomID uint, roomID uint) (*dto.RoomActivityResponse, error)
	StartRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, error)
	JoinRoom(ctx context.Context, userID string, roomID uint) (*dto.JoinRoomResponse, error)
	EndRoom(ctx context.Context, teacherID string, roomID uint) (*dto.RoomResponse, error)
	GetRoomAttendance(ctx context.Context, userID string, roomID uint) ([]*dto.AttendanceSessionResponse, error)
	HandleParticipantJoined(ctx context.Context, roomName, userID string, joinedAt time.Time) error
	HandleParticipantLeft(ctx context.Context, roomName, userID string, leftAt time.Time) error
	ProcessRoomLifecycle(ctx context.Context) (int, error)
	SetMediaClient(media.Client)
	SetRoomEventPublisher(func(string, any))
}

type service struct {
	repo                *repository.Repository
	cfg                 *classroomsConfig.AppConfig
	usersClient         usersClient.UserServiceClient
	examsClient         examsClient.ExamServiceClient
	notificationsClient notificationsClient.NotificationServiceClient
	mediaClient         media.Client
	roomPublisher       func(string, any)
}

func (s *service) SetNotificationClient(client notificationsClient.NotificationServiceClient) {
	s.notificationsClient = client
}
func (s *service) SetMediaClient(client media.Client)                { s.mediaClient = client }
func (s *service) SetRoomEventPublisher(publisher func(string, any)) { s.roomPublisher = publisher }

func New(repo *repository.Repository, appConfig *classroomsConfig.AppConfig, usersServiceClient usersClient.UserServiceClient, examsServiceClient examsClient.ExamServiceClient) Service {
	return &service{
		repo:        repo,
		cfg:         appConfig,
		usersClient: usersServiceClient,
		examsClient: examsServiceClient,
	}
}

var (
	ErrClassroomNotActive             = errors.New("[ERROR] Classroom is not active")
	ErrClassroomForbidden             = errors.New("[FORBIDDEN] Classroom access denied")
	ErrClassroomCapacityFull          = errors.New("[ERROR] Classroom capacity is full")
	ErrTeacherCannotJoinClass         = errors.New("[ERROR] Teacher cannot join own classroom as student")
	ErrAssignmentInvalidTime          = errors.New("[ERROR] Assignment due time must be after open time")
	ErrAssignmentDuration             = errors.New("[ERROR] Assignment duration must not exceed the open time window")
	ErrAssignmentExpired              = errors.New("[ERROR] An expired assignment cannot be activated")
	ErrAssignmentAlreadyExists        = errors.New("[CONFLICT] This exam is already assigned to the classroom")
	ErrAssignmentScheduleConfirmation = errors.New("[CONFIRMATION_REQUIRED] Active attempts may be affected by this schedule change")
	ErrAssignmentContentConfirmation  = errors.New("[CONFIRMATION_REQUIRED] This exam already has attempts; confirm to submit active attempts and pause all related assignments")
	ErrScheduleInvalidTime            = errors.New("[ERROR] Schedule end time must be after start time")
	ErrCalendarInvalidRange           = errors.New("[ERROR] Calendar range must be valid and no longer than 366 days")
	ErrRoomLive                       = errors.New("[CONFLICT] The room is currently live")
	ErrRoomNotLive                    = errors.New("[ERROR] This session has not started yet")
	ErrRoomEnded                      = errors.New("[ERROR] This session has ended")
	ErrRoomTooEarly                   = errors.New("[ERROR] The room can only be started up to 30 minutes before the scheduled time")
	ErrMediaUnavailable               = errors.New("[UNAVAILABLE] The media server is temporarily unavailable")
)

func (s *service) CreateSchedule(ctx context.Context, teacherID string, classroomID uint, req dto.CreateScheduleRequest) (*dto.CalendarResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !req.EndsAt.After(req.StartsAt) {
		return nil, ErrScheduleInvalidTime
	}
	schedule := &model.Schedule{ClassroomID: classroomID, Title: strings.TrimSpace(req.Title), Description: req.Description, StartsAt: req.StartsAt, EndsAt: req.EndsAt}
	if schedule.Description != nil {
		value := strings.TrimSpace(*schedule.Description)
		schedule.Description = &value
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.CreateSchedule(ctx, schedule); err != nil {
			return err
		}
		room := &model.Room{ScheduleID: schedule.ID, Status: model.RoomStatusScheduled, Schedule: *schedule}
		if err := repo.CreateRoom(ctx, room); err != nil {
			return err
		}
		schedule.Room = room
		return nil
	}); err != nil {
		return nil, err
	}
	return dto.ToCalendarResponse(schedule), nil
}

func (s *service) GetCalendar(ctx context.Context, userID string, classroomID uint, query dto.CalendarQuery) ([]*dto.CalendarResponse, error) {
	if !query.To.After(query.From) || query.To.Sub(query.From) > 366*24*time.Hour {
		return nil, ErrCalendarInvalidRange
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != userID {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	schedules, err := s.repo.GetSchedulesByRange(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, err
	}
	assignments, err := s.repo.GetAssignmentsDueInRange(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.CalendarResponse, 0, len(schedules)+len(assignments))
	for _, schedule := range schedules {
		responses = append(responses, dto.ToCalendarResponse(schedule))
	}
	for _, assignment := range assignments {
		assignmentID := assignment.ID
		responses = append(responses, &dto.CalendarResponse{ClassroomID: classroomID, CalendarType: dto.CalendarTypeAssignmentDeadline, Title: "Deadline: " + assignment.Title, Description: assignment.Description, StartsAt: assignment.DueAt, RelatedAssignmentID: &assignmentID, Editable: false})
	}
	sort.SliceStable(responses, func(i, j int) bool {
		if responses[i].StartsAt.Equal(responses[j].StartsAt) {
			return responses[i].ID < responses[j].ID
		}
		return responses[i].StartsAt.Before(responses[j].StartsAt)
	})
	return responses, nil
}

func (s *service) UpdateSchedule(ctx context.Context, teacherID string, scheduleID uint, req dto.UpdateScheduleRequest) (*dto.CalendarResponse, error) {
	schedule, err := s.repo.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return nil, err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	if !req.EndsAt.After(req.StartsAt) {
		return nil, ErrScheduleInvalidTime
	}
	schedule.Title, schedule.Description = strings.TrimSpace(req.Title), req.Description
	schedule.StartsAt, schedule.EndsAt = req.StartsAt, req.EndsAt
	if schedule.Description != nil {
		value := strings.TrimSpace(*schedule.Description)
		schedule.Description = &value
	}
	if err := s.repo.UpdateSchedule(ctx, schedule); err != nil {
		return nil, err
	}
	return dto.ToCalendarResponse(schedule), nil
}

func (s *service) DeleteSchedule(ctx context.Context, teacherID string, scheduleID uint) error {
	schedule, err := s.repo.GetScheduleByID(ctx, scheduleID)
	if err != nil {
		return err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
	if err != nil {
		return err
	}
	if classroom.TeacherID != teacherID {
		return ErrClassroomForbidden
	}
	if !classroom.Active {
		return ErrClassroomNotActive
	}
	if schedule.Room != nil && schedule.Room.Status == model.RoomStatusLive {
		return ErrRoomLive
	}
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if schedule.Room != nil {
			if err := repo.DeleteRoom(ctx, schedule.Room); err != nil {
				return err
			}
		}
		return repo.DeleteSchedule(ctx, schedule)
	})
}

func (s *service) CreateClassroom(ctx context.Context, teacherID string, req dto.CreateClassroomRequest) (*dto.CreatedClassroomResponse, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const codeLength = 8
	const maxCodeAttempts = 5

	// Thử sinh mã lớp tối đa một số lần hữu hạn để xử lý trường hợp mã ngẫu nhiên bị trùng.
	for codeAttempt := 0; codeAttempt < maxCodeAttempts; codeAttempt++ {
		// Dùng nguồn ngẫu nhiên mật mã và loại các ký tự dễ nhầm lẫn khi học sinh nhập mã lớp.
		codeBytes := make([]byte, codeLength)
		for i := range codeBytes {
			index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return nil, err
			}
			codeBytes[i] = alphabet[index.Int64()]
		}

		var response *dto.CreatedClassroomResponse
		// Chỉ ghi nhận lớp khi cả dữ liệu lớp và thông tin giáo viên dùng cho response đều lấy thành công.
		err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
			classroom := &model.Classroom{
				Code:        string(codeBytes),
				Name:        strings.TrimSpace(req.Name),
				Description: req.Description,
				Avatar:      req.Avatar,
				TeacherID:   teacherID,
				Active:      true,
			}
			if err := repo.CreateClassroom(ctx, classroom); err != nil {
				return err
			}

			response = dto.ToCreatedClassroomResponse(classroom)
			// Lấy thông tin hiển thị của giáo viên từ Users thay vì sao chép dữ liệu người dùng sang Classrooms.
			usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{teacherID}})
			if err != nil {
				return err
			}
			usersByID := dto.ToUserInfoMap(usersResp.Users)
			response.Teacher = usersByID[teacherID]
			return nil
		})
		if err == nil {
			return response, nil
		}
		// Chỉ sinh mã mới khi vi phạm unique; các lỗi khác phải được trả về ngay.
		if !platformdb.IsUniqueViolation(err, "") {
			return nil, err
		}
	}
	return nil, errors.New("[ERROR] Could not generate a unique classroom code")
}

func (s *service) GetClassrooms(ctx context.Context, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error) {
	var teacherIDs []string
	if query.Query != "" {
		usersResp, err := s.usersClient.SearchUserIDsByName(ctx, &usersClient.SearchUserIDsByNameRequest{Query: query.Query})
		if err != nil {
			return nil, 0, err
		}
		teacherIDs = usersResp.Ids
	}
	classrooms, total, err := s.repo.GetClassrooms(ctx, query, teacherIDs)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]string, 0, len(classrooms))
	seenUserIDs := make(map[string]struct{}, len(classrooms))
	for _, classroom := range classrooms {
		if _, ok := seenUserIDs[classroom.TeacherID]; !ok {
			seenUserIDs[classroom.TeacherID] = struct{}{}
			userIDs = append(userIDs, classroom.TeacherID)
		}
		for _, member := range classroom.Members {
			if _, ok := seenUserIDs[member.UserID]; !ok {
				seenUserIDs[member.UserID] = struct{}{}
				userIDs = append(userIDs, member.UserID)
			}
		}
	}
	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, err
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetMyClassrooms(ctx context.Context, userID string, query dto.ClassroomsQuery) ([]*dto.ClassroomResponse, int, error) {
	var teacherIDs []string
	if query.Query != "" {
		usersResp, err := s.usersClient.SearchUserIDsByName(ctx, &usersClient.SearchUserIDsByNameRequest{Query: query.Query})
		if err != nil {
			return nil, 0, err
		}
		teacherIDs = usersResp.Ids
	}
	classrooms, total, err := s.repo.GetClassroomsByUser(ctx, userID, query, teacherIDs)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]string, 0, len(classrooms))
	seenUserIDs := make(map[string]struct{}, len(classrooms))
	for _, classroom := range classrooms {
		if _, ok := seenUserIDs[classroom.TeacherID]; !ok {
			seenUserIDs[classroom.TeacherID] = struct{}{}
			userIDs = append(userIDs, classroom.TeacherID)
		}
		for _, member := range classroom.Members {
			if _, ok := seenUserIDs[member.UserID]; !ok {
				seenUserIDs[member.UserID] = struct{}{}
				userIDs = append(userIDs, member.UserID)
			}
		}
	}
	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, err
		}
		users = usersResp.Users
	}
	responses := dto.ToClassroomResponses(classrooms, users)
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetClassroom(ctx context.Context, userID string, id uint) (*dto.ClassroomResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != userID {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, id, userID)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}
	response := dto.ToClassroomResponse(classroom)
	userIDs := []string{classroom.TeacherID}
	for _, member := range classroom.Members {
		userIDs = append(userIDs, member.UserID)
	}
	usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
	if err != nil {
		return nil, err
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	for i := range classroom.Members {
		response.Members[i].User = usersByID[classroom.Members[i].UserID]
	}
	return response, nil
}

func (s *service) GetClassroomByCode(ctx context.Context, code string) (*dto.ClassroomByCodeResponse, error) {
	classroom, err := s.repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, err
	}
	if !classroom.Active {
		return nil, ErrClassroomNotActive
	}
	countApprovedMembers, err := s.repo.CountApprovedMembers(ctx, classroom.ID)
	if err != nil {
		return nil, err
	}
	response := dto.ToClassroomByCodeResponse(classroom, countApprovedMembers)
	usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
	if err != nil {
		return nil, err
	}
	usersByID := dto.ToUserInfoMap(usersResp.Users)
	response.Teacher = usersByID[classroom.TeacherID]
	return response, nil
}

func (s *service) UpdateClassroom(ctx context.Context, teacherID string, id uint, req dto.UpdateClassroomRequest) (*dto.ClassroomResponse, error) {
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, id)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if req.Name != nil {
			classroom.Name = strings.TrimSpace(*req.Name)
		}
		if req.Description != nil {
			classroom.Description = req.Description
		}
		if req.Avatar != nil {
			classroom.Avatar = req.Avatar
		}
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) ArchiveClassroom(ctx context.Context, teacherID string, id uint) (*dto.ClassroomResponse, error) {
	var response *dto.ClassroomResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, id)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		live, err := repo.HasLiveRoomByClassroom(ctx, id)
		if err != nil {
			return err
		}
		if live {
			return ErrRoomLive
		}
		classroom.Active = false
		if err := repo.UpdateClassroom(ctx, classroom); err != nil {
			return err
		}
		response = dto.ToClassroomResponse(classroom)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.Teacher = usersByID[classroom.TeacherID]
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RequestJoinClassroom(ctx context.Context, userID string, req dto.RequestJoinClassroomRequest) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByCode(ctx, strings.ToUpper(strings.TrimSpace(req.Code)))
		if err != nil {
			return err
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		count, err := repo.CountApprovedMembers(ctx, classroom.ID)
		if err != nil {
			return err
		}
		if count >= int64(s.cfg.MaxStudentsPerClass) {
			return ErrClassroomCapacityFull
		}
		if existing, err := repo.GetMember(ctx, classroom.ID, userID); err == nil {
			if existing.Approved != nil && !*existing.Approved {
				existing.Approved = nil
				existing.ApprovedAt = nil
				existing.JoinedAt = nil
				if err := repo.UpsertMember(ctx, existing); err != nil {
					return err
				}
			}
			existing.Classroom = *classroom
			response = dto.ToClassroomJoinRequestResponse(existing)
			usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{existing.UserID, classroom.TeacherID}})
			if err != nil {
				return err
			}
			usersByID := dto.ToUserInfoMap(usersResp.Users)
			response.User = usersByID[existing.UserID]
			response.Classroom.Teacher = usersByID[classroom.TeacherID]
			if err := createNotificationOutbox(ctx, repo,
				fmt.Sprintf("classroom:%d:join-request:%d:%d", classroom.ID, existing.ID, existing.UpdatedAt.UnixNano()),
				[]string{classroom.TeacherID}, "New join request", "A student requested to join "+classroom.Name,
				"CLASSROOM", classroom.ID, 0, time.Time{}); err != nil {
				return err
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		member := &model.Member{
			ClassroomID: classroom.ID,
			UserID:      userID,
			Approved:    nil,
			ApprovedAt:  nil,
			JoinedAt:    nil,
		}
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		member.Classroom = *classroom
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo,
			fmt.Sprintf("classroom:%d:join-request:%d:%d", classroom.ID, member.ID, member.UpdatedAt.UnixNano()),
			[]string{classroom.TeacherID}, "New join request", "A student requested to join "+classroom.Name,
			"CLASSROOM", classroom.ID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) GetJoinRequests(ctx context.Context, teacherID string, classroomID uint) ([]*dto.ClassroomJoinRequestResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	members, err := s.repo.GetJoinRequests(ctx, classroomID)
	if err != nil {
		return nil, err
	}

	responses := make([]*dto.ClassroomJoinRequestResponse, len(members))
	userIDs := make([]string, 0, len(members)*2)
	seenUserIDs := make(map[string]struct{}, len(members)*2)
	for i, member := range members {
		responses[i] = dto.ToClassroomJoinRequestResponse(member)
		if _, ok := seenUserIDs[member.UserID]; !ok {
			seenUserIDs[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
		if member.Classroom.TeacherID != "" {
			if _, ok := seenUserIDs[member.Classroom.TeacherID]; !ok {
				seenUserIDs[member.Classroom.TeacherID] = struct{}{}
				userIDs = append(userIDs, member.Classroom.TeacherID)
			}
		}
	}
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		for i, response := range responses {
			response.User = usersByID[members[i].UserID]
			if response.Classroom != nil {
				response.Classroom.Teacher = usersByID[members[i].Classroom.TeacherID]
			}
		}
	}

	return responses, nil
}

func (s *service) ApproveMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		if classroom.TeacherID == userID {
			return ErrTeacherCannotJoinClass
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := true
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = &now
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("classroom:%d:member:%d:approved", classroomID, member.ID),
			[]string{userID}, "Join request approved", "You are now a member of "+classroom.Name,
			"CLASSROOM", classroomID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RejectMember(ctx context.Context, teacherID string, classroomID uint, userID string) (*dto.ClassroomJoinRequestResponse, error) {
	var response *dto.ClassroomJoinRequestResponse
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if !classroom.Active {
			return ErrClassroomNotActive
		}
		member, err := repo.GetMember(ctx, classroomID, userID)
		if err != nil {
			return err
		}
		approved := false
		now := time.Now()
		member.Approved = &approved
		member.ApprovedAt = &now
		member.JoinedAt = nil
		if err := repo.UpsertMember(ctx, member); err != nil {
			return err
		}
		response = dto.ToClassroomJoinRequestResponse(member)
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: []string{member.UserID, member.Classroom.TeacherID}})
		if err != nil {
			return err
		}
		usersByID := dto.ToUserInfoMap(usersResp.Users)
		response.User = usersByID[member.UserID]
		response.Classroom.Teacher = usersByID[member.Classroom.TeacherID]
		if err := createNotificationOutbox(ctx, repo,
			fmt.Sprintf("classroom:%d:member:%d:rejected:%d", classroomID, member.ID, member.UpdatedAt.UnixNano()),
			[]string{userID}, "Join request declined", "Your request to join "+classroom.Name+" was declined",
			"CLASSROOM", classroomID, 0, time.Time{}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *service) RemoveMember(ctx context.Context, teacherID string, classroomID uint, userID string) error {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return err
	}
	if classroom.TeacherID != teacherID {
		return ErrClassroomForbidden
	}
	member, err := s.repo.GetMember(ctx, classroomID, userID)
	if err != nil {
		return err
	}
	previousApproved := member.Approved
	previousApprovedAt := member.ApprovedAt
	revoked := false
	member.Approved = &revoked
	member.ApprovedAt = nil
	if err := s.repo.UpsertMember(ctx, member); err != nil {
		return err
	}
	assignmentIDs, err := s.repo.GetAssignmentIDsByClassroom(ctx, classroomID)
	if err != nil {
		member.Approved = previousApproved
		member.ApprovedAt = previousApprovedAt
		_ = s.repo.UpsertMember(ctx, member)
		return err
	}
	grpcAssignmentIDs := make([]uint64, len(assignmentIDs))
	for i, id := range assignmentIDs {
		grpcAssignmentIDs[i] = uint64(id)
	}
	if _, err := s.examsClient.SubmitStudentAssignmentAttempts(ctx, &examsClient.SubmitStudentAssignmentAttemptsRequest{StudentId: userID, AssignmentIds: grpcAssignmentIDs}); err != nil {
		member.Approved = previousApproved
		member.ApprovedAt = previousApprovedAt
		if rollbackErr := s.repo.UpsertMember(ctx, member); rollbackErr != nil {
			return errors.Join(err, rollbackErr)
		}
		return err
	}
	if s.mediaClient != nil {
		rooms, err := s.repo.GetLiveRoomsByClassroom(ctx, classroomID)
		if err != nil {
			return err
		}
		for _, room := range rooms {
			_ = s.mediaClient.RemoveParticipant(ctx, media.RoomName(room.ID), userID)
		}
	}
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.DeleteMember(ctx, member); err != nil {
			return err
		}
		return createNotificationOutbox(ctx, repo,
			fmt.Sprintf("classroom:%d:member:%d:removed:%d", classroomID, member.ID, time.Now().UnixNano()),
			[]string{userID}, "Removed from classroom", "You were removed from "+classroom.Name,
			"CLASSROOM", classroomID, 0, time.Time{})
	})
}

func (s *service) CreateAssignments(ctx context.Context, teacherID string, req dto.CreateAssignmentsRequest) (*dto.CreatedAssignmentsResponse, error) {
	// Kiểm tra toàn bộ lịch, lớp trùng và quyền sở hữu trước khi tạo Exam ở service khác.
	// Việc kiểm tra sớm giúp hạn chế tạo ra Exam mồ côi khi request không hợp lệ.
	seenClassroomIDs := make(map[uint]struct{}, len(req.Schedules))
	for _, schedule := range req.Schedules {
		if _, ok := seenClassroomIDs[schedule.ClassroomID]; ok {
			return nil, ErrAssignmentAlreadyExists
		}
		seenClassroomIDs[schedule.ClassroomID] = struct{}{}
		if !schedule.DueAt.After(schedule.OpensAt) {
			return nil, ErrAssignmentInvalidTime
		}
		if time.Duration(schedule.Duration)*time.Minute > schedule.DueAt.Sub(schedule.OpensAt) {
			return nil, ErrAssignmentDuration
		}
		classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
		if err != nil {
			return nil, err
		}
		if classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		if !classroom.Active {
			return nil, ErrClassroomNotActive
		}
	}

	// Chuyển dữ liệu câu hỏi sang contract gRPC của Exams và chuẩn hóa các chuỗi nhập vào.
	examReq := &examsClient.CreatePrivateExamRequest{
		Title:     strings.TrimSpace(req.Title),
		Questions: make([]*examsClient.CreatePrivateExamQuestion, len(req.Questions)),
	}
	if req.Description != nil {
		examReq.Description = req.Description
	}
	for i, question := range req.Questions {
		examQuestion := &examsClient.CreatePrivateExamQuestion{
			Content:            strings.TrimSpace(question.Content),
			Options:            make([]string, len(question.Options)),
			CorrectOptionIndex: int32(question.CorrectOptionIndex),
		}
		if question.Explanation != nil {
			examQuestion.Explanation = question.Explanation
		}
		for optionIndex, option := range question.Options {
			examQuestion.Options[optionIndex] = strings.TrimSpace(option)
		}
		examReq.Questions[i] = examQuestion
	}

	// Exams chịu trách nhiệm tạo toàn bộ cây Exam private, section, câu hỏi và lựa chọn trong transaction riêng.
	exam, err := s.examsClient.CreatePrivateExam(ctx, examReq)
	if err != nil {
		return nil, err
	}

	var assignments []*model.Assignment
	// Tạo tất cả Assignment trong cùng transaction để không xảy ra trạng thái chỉ một phần lớp được giao bài.
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		assignments = make([]*model.Assignment, 0, len(req.Schedules))
		for _, schedule := range req.Schedules {
			assignments = append(assignments, &model.Assignment{
				ClassroomID: schedule.ClassroomID,
				Title:       strings.TrimSpace(req.Title),
				Description: req.Description,
				ExamID:      uint(exam.Id),
				Duration:    schedule.Duration,
				OpensAt:     schedule.OpensAt,
				DueAt:       schedule.DueAt,
				Active:      true,
			})
		}

		// Ghi toàn bộ Assignment sau khi mọi lớp đều đã vượt qua bước kiểm tra.
		if err := repo.CreateAssignments(ctx, assignments); err != nil {
			return err
		}
		for _, assignment := range assignments {
			members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
			if err != nil {
				return err
			}
			recipients := make([]string, len(members))
			for i, member := range members {
				recipients[i] = member.UserID
			}
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:created", assignment.ID), recipients,
				"New assignment", assignment.Title, "ASSIGNMENT", assignment.ID, assignment.ClassroomID, time.Time{}); err != nil {
				return err
			}
			reminderAt := assignment.DueAt.Add(-24 * time.Hour)
			if reminderAt.Before(time.Now()) {
				reminderAt = time.Now()
			}
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// Classrooms không thể rollback transaction của Exams, vì vậy phải gọi RPC bù để xóa Exam vừa tạo.
		if _, rollbackErr := s.examsClient.DeletePrivateExamRollback(ctx, &examsClient.DeletePrivateExamRollbackRequest{Id: exam.Id}); rollbackErr != nil {
			logger.Log.Error().Err(rollbackErr).Uint64("exam_id", exam.Id).Msg("[CLASSROOMS] Failed to rollback private exam after assignment creation failed")
		}
		return nil, err
	}

	// Chỉ trả response khi Exam và toàn bộ Assignment đều đã được tạo thành công.
	return dto.ToCreatedAssignmentsResponse(assignments), nil
}

func (s *service) AssignExistingExam(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.AssignExistingExamRequest) (*dto.CreatedAssignmentsResponse, error) {
	source, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	sourceClassroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if sourceClassroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}

	seen := make(map[uint]struct{}, len(req.Schedules))
	for _, schedule := range req.Schedules {
		if _, ok := seen[schedule.ClassroomID]; ok {
			return nil, ErrAssignmentAlreadyExists
		}
		seen[schedule.ClassroomID] = struct{}{}
		if !schedule.DueAt.After(schedule.OpensAt) {
			return nil, ErrAssignmentInvalidTime
		}
		if time.Duration(schedule.Duration)*time.Minute > schedule.DueAt.Sub(schedule.OpensAt) {
			return nil, ErrAssignmentDuration
		}
		classroom, err := s.repo.GetClassroomByID(ctx, schedule.ClassroomID)
		if err != nil {
			return nil, err
		}
		if classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		if !classroom.Active {
			return nil, ErrClassroomNotActive
		}
		exists, err := s.repo.AssignmentExistsByClassroomAndExam(ctx, schedule.ClassroomID, source.ExamID)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrAssignmentAlreadyExists
		}
	}

	var assignments []*model.Assignment
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		assignments = make([]*model.Assignment, 0, len(req.Schedules))
		for _, schedule := range req.Schedules {
			assignments = append(assignments, &model.Assignment{
				ClassroomID: schedule.ClassroomID,
				Title:       source.Title,
				Description: source.Description,
				ExamID:      source.ExamID,
				Duration:    schedule.Duration,
				OpensAt:     schedule.OpensAt,
				DueAt:       schedule.DueAt,
				Active:      true,
			})
		}
		if err := repo.CreateAssignments(ctx, assignments); err != nil {
			return err
		}
		for _, assignment := range assignments {
			members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
			if err != nil {
				return err
			}
			recipients := make([]string, len(members))
			for i, member := range members {
				recipients[i] = member.UserID
			}
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:created", assignment.ID), recipients,
				"New assignment", assignment.Title, "ASSIGNMENT", assignment.ID, assignment.ClassroomID, time.Time{}); err != nil {
				return err
			}
			reminderAt := assignment.DueAt.Add(-24 * time.Hour)
			if reminderAt.Before(time.Now()) {
				reminderAt = time.Now()
			}
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dto.ToCreatedAssignmentsResponse(assignments), nil
}

func (s *service) UpdateAssignmentSchedule(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentScheduleRequest) (*dto.AssignmentResponse, error) {
	if !req.DueAt.After(req.OpensAt) {
		return nil, ErrAssignmentInvalidTime
	}
	if time.Duration(req.Duration)*time.Minute > req.DueAt.Sub(req.OpensAt) {
		return nil, ErrAssignmentDuration
	}

	// Lấy trạng thái attempt từ Exams để xác định thay đổi lịch có cần giáo viên xác nhận hay không.
	state, err := s.examsClient.GetAssignmentAttemptState(ctx, &examsClient.GetAssignmentAttemptStateRequest{AssignmentId: uint64(assignmentID)})
	if err != nil {
		return nil, err
	}
	var response *dto.AssignmentResponse
	var previous *model.Assignment
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		assignment, err := repo.GetAssignmentByClassroomForUpdate(ctx, classroomID, assignmentID)
		if err != nil {
			return err
		}
		if state.TotalCount > 0 && !req.Confirm && (req.DueAt.Before(assignment.DueAt) || req.Duration < assignment.Duration) {
			return ErrAssignmentScheduleConfirmation
		}

		// Giữ bản sao trước khi cập nhật vì transaction của Classrooms không thể bao phủ database của Exams.
		copy := *assignment
		previous = &copy
		assignment.Duration = req.Duration
		assignment.OpensAt = req.OpensAt
		assignment.DueAt = req.DueAt
		if err := repo.UpdateAssignment(ctx, assignment); err != nil {
			return err
		}
		if err := repo.CancelNotificationOutbox(ctx, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID)); err != nil {
			return err
		}
		members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
		if err != nil {
			return err
		}
		recipients := make([]string, len(members))
		for i, member := range members {
			recipients[i] = member.UserID
		}
		reminderAt := assignment.DueAt.Add(-24 * time.Hour)
		if reminderAt.Before(time.Now()) {
			reminderAt = time.Now()
		}
		if assignment.Active {
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:due-reminder:%d", assignment.ID, assignment.UpdatedAt.UnixNano()), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		response = dto.ToAssignmentResponse(assignment, time.Now(), "", true, nil)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Đồng bộ thời lượng và hạn nộp sang các attempt hiện có bên Exams.
	if _, err := s.examsClient.UpdateAssignmentAttemptSchedule(ctx, &examsClient.UpdateAssignmentAttemptScheduleRequest{
		AssignmentId: uint64(assignmentID), DurationMinutes: int32(req.Duration), DueAtUnix: req.DueAt.Unix(),
	}); err != nil {
		// Khôi phục lịch cũ ở Classrooms nếu Exams không nhận được lịch mới.
		if previous != nil {
			if rollbackErr := s.repo.UpdateAssignment(ctx, previous); rollbackErr != nil {
				logger.Log.Error().Err(rollbackErr).Uint("assignment_id", assignmentID).
					Msg("[CLASSROOMS] Failed to restore assignment schedule after Exams synchronization failed")
			}
		}
		return nil, err
	}
	return response, nil
}

func (s *service) SetAssignmentActive(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, active bool) (*dto.AssignmentResponse, error) {
	var response *dto.AssignmentResponse
	var previousActive bool

	// Khóa Assignment trong transaction để kiểm tra quyền và cập nhật trạng thái nhất quán ở Classrooms.
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		classroom, err := repo.GetClassroomByID(ctx, classroomID)
		if err != nil {
			return err
		}
		if classroom.TeacherID != teacherID {
			return ErrClassroomForbidden
		}
		if active && !classroom.Active {
			return ErrClassroomNotActive
		}

		// Lấy Assignment kèm khóa ghi để tránh hai request bật/tắt cập nhật đè lên nhau.
		assignment, err := repo.GetAssignmentByClassroomForUpdate(ctx, classroomID, assignmentID)
		if err != nil {
			return err
		}
		if active && !assignment.DueAt.After(time.Now()) {
			return ErrAssignmentExpired
		}

		// Lưu trạng thái cũ để có thể khôi phục nếu thao tác liên service phía sau thất bại.
		previousActive = assignment.Active
		assignment.Active = active
		if err := repo.UpdateAssignment(ctx, assignment); err != nil {
			return err
		}
		if !active {
			if err := repo.CancelNotificationOutbox(ctx, fmt.Sprintf("assignment:%d:due-reminder", assignment.ID)); err != nil {
				return err
			}
		} else {
			members, err := repo.GetApprovedMembersByClassroom(ctx, assignment.ClassroomID)
			if err != nil {
				return err
			}
			recipients := make([]string, len(members))
			for i, member := range members {
				recipients[i] = member.UserID
			}
			reminderAt := assignment.DueAt.Add(-24 * time.Hour)
			if reminderAt.Before(time.Now()) {
				reminderAt = time.Now()
			}
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("assignment:%d:due-reminder:%d", assignment.ID, assignment.UpdatedAt.UnixNano()), recipients,
				"Assignment due soon", assignment.Title+" is due soon", "ASSIGNMENT", assignment.ID, assignment.ClassroomID, reminderAt); err != nil {
				return err
			}
		}
		response = dto.ToAssignmentResponse(assignment, time.Now(), "", true, nil)
		return nil
	}); err != nil {
		return nil, err
	}

	// Khi tắt Assignment, yêu cầu Exams nộp ngay mọi attempt đang làm bằng các đáp án đã lưu.
	if !active {
		if _, err := s.examsClient.SubmitAssignmentAttempts(ctx, &examsClient.SubmitAssignmentAttemptsRequest{AssignmentId: uint64(assignmentID)}); err != nil {
			// Nếu Assignment trước đó đang bật, khôi phục lại để tránh trạng thái đã tắt nhưng attempt chưa được xử lý.
			if previousActive {
				rollbackErr := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
					assignment, rollbackErr := repo.GetAssignmentByClassroomForUpdate(ctx, classroomID, assignmentID)
					if rollbackErr != nil {
						return rollbackErr
					}
					assignment.Active = true
					return repo.UpdateAssignment(ctx, assignment)
				})
				if rollbackErr != nil {
					logger.Log.Error().Err(rollbackErr).Uint("assignment_id", assignmentID).
						Msg("[CLASSROOMS] Failed to restore assignment after Exams force-submit failed")
					return nil, errors.Join(err, rollbackErr)
				}
			}
			return nil, err
		}
	}
	return response, nil
}

func (s *service) GetEditableAssignment(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.EditableAssignmentResponse, error) {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	exam, err := s.examsClient.GetPrivateExam(ctx, &examsClient.GetPrivateExamRequest{Id: uint64(assignment.ExamID)})
	if err != nil {
		return nil, err
	}
	relatedAssignments, err := s.repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
	if err != nil {
		return nil, err
	}
	hasAttempts := false
	var activeAttempts int64
	var submittedAttempts int64
	affected := make([]*dto.AffectedAssignmentResponse, 0, len(relatedAssignments))
	for _, related := range relatedAssignments {
		state, err := s.examsClient.GetAssignmentAttemptState(ctx, &examsClient.GetAssignmentAttemptStateRequest{AssignmentId: uint64(related.ID)})
		if err != nil {
			return nil, err
		}
		activeAttempts += state.ActiveCount
		submittedAttempts += state.TotalCount - state.ActiveCount
		hasAttempts = hasAttempts || state.TotalCount > 0
		affected = append(affected, &dto.AffectedAssignmentResponse{ID: related.ID, ClassroomID: related.ClassroomID, ClassroomName: related.Classroom.Name, Active: related.Active})
	}
	response := &dto.EditableAssignmentResponse{ID: assignment.ID, ExamID: assignment.ExamID, Title: exam.Title, Description: exam.Description, Questions: make([]*dto.EditableAssignmentQuestion, len(exam.Questions)), HasAttempts: hasAttempts, ActiveAttempts: activeAttempts, SubmittedAttempts: submittedAttempts, AffectedAssignments: affected}
	for i, question := range exam.Questions {
		optionIDs := make([]uint, len(question.OptionIds))
		for j, id := range question.OptionIds {
			optionIDs[j] = uint(id)
		}
		response.Questions[i] = &dto.EditableAssignmentQuestion{ID: uint(question.Id), Content: question.Content, Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: int(question.CorrectOptionIndex), ExcludedFromScore: question.ExcludedFromScore, OptionIDs: optionIDs}
	}
	return response, nil
}

func (s *service) ResetAssignmentAttempt(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, studentID string) error {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return err
	}
	if classroom.TeacherID != teacherID {
		return ErrClassroomForbidden
	}
	if _, err := s.repo.GetAssignmentSubmissionByAssignmentAndStudent(ctx, assignment.ID, studentID); err != nil {
		return err
	}
	if _, err := s.examsClient.ResetAssignmentAttempt(ctx, &examsClient.ResetAssignmentAttemptRequest{AssignmentId: uint64(assignment.ID), StudentId: studentID}); err != nil {
		return err
	}
	return s.repo.DeleteAssignmentSubmission(ctx, assignment.ID, studentID)
}

func (s *service) ReconcileAssignmentSubmissions(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (int, error) {
	if _, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID); err != nil {
		return 0, err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return 0, err
	}
	if classroom.TeacherID != teacherID {
		return 0, ErrClassroomForbidden
	}
	response, err := s.examsClient.ReconcileAssignmentSubmissions(ctx, &examsClient.ReconcileAssignmentSubmissionsRequest{AssignmentId: uint64(assignmentID)})
	if err != nil {
		return 0, err
	}
	return int(response.EnqueuedCount), nil
}

func (s *service) GetAssignmentAudits(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) ([]*dto.ExamAuditResponse, error) {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	audits, err := s.examsClient.GetExamAudits(ctx, &examsClient.GetExamAuditsRequest{ExamId: uint64(assignment.ExamID)})
	if err != nil {
		return nil, err
	}
	responses := make([]*dto.ExamAuditResponse, len(audits.Audits))
	actorIDs := make([]string, 0, len(audits.Audits))
	seenActorIDs := make(map[string]struct{}, len(audits.Audits))
	for i, audit := range audits.Audits {
		item := &dto.ExamAuditResponse{ID: uint(audit.Id), ExamID: uint(audit.ExamId), ActorID: audit.ActorId, Reason: audit.Reason, Before: audit.Before, After: audit.After, CreatedAt: time.Unix(audit.CreatedAtUnix, 0), ScoreChanges: make([]*dto.ExamAuditScoreChangeResponse, len(audit.ScoreChanges))}
		if audit.RegradedAtUnix != nil {
			value := time.Unix(*audit.RegradedAtUnix, 0)
			item.RegradedAt = &value
		}
		for j, change := range audit.ScoreChanges {
			item.ScoreChanges[j] = &dto.ExamAuditScoreChangeResponse{AttemptID: uint(change.AttemptId), StudentID: change.StudentId, BeforeScore: change.BeforeScore, AfterScore: change.AfterScore}
		}
		responses[i] = item
		if _, exists := seenActorIDs[audit.ActorId]; !exists {
			seenActorIDs[audit.ActorId] = struct{}{}
			actorIDs = append(actorIDs, audit.ActorId)
		}
	}
	if len(actorIDs) > 0 {
		users, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: actorIDs})
		if err != nil {
			return nil, err
		}
		usersByID := dto.ToUserInfoMap(users.Users)
		for _, response := range responses {
			response.Actor = usersByID[response.ActorID]
		}
	}
	return responses, nil
}

func (s *service) UpdateAssignmentContent(ctx context.Context, teacherID string, classroomID uint, assignmentID uint, req dto.UpdateAssignmentContentRequest) (*dto.EditableAssignmentResponse, error) {
	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	assignments, err := s.repo.GetAssignmentsByExamID(ctx, assignment.ExamID)
	if err != nil {
		return nil, err
	}
	activeStates := make(map[uint]bool, len(assignments))
	var totalAttempts int64
	for _, related := range assignments {
		if related.Classroom.TeacherID != teacherID {
			return nil, ErrClassroomForbidden
		}
		activeStates[related.ID] = related.Active
		state, err := s.examsClient.GetAssignmentAttemptState(ctx, &examsClient.GetAssignmentAttemptStateRequest{AssignmentId: uint64(related.ID)})
		if err != nil {
			return nil, err
		}
		totalAttempts += state.TotalCount
	}
	if totalAttempts > 0 && !req.Confirm {
		return nil, ErrAssignmentContentConfirmation
	}
	if totalAttempts > 0 && strings.TrimSpace(req.Reason) == "" {
		return nil, errors.New("[ERROR] A reason is required when changing an exam with attempts")
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		return repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, assignment.Title, assignment.Description, false)
	}); err != nil {
		return nil, err
	}
	if totalAttempts > 0 {
		for _, related := range assignments {
			if _, err := s.examsClient.SubmitAssignmentAttempts(ctx, &examsClient.SubmitAssignmentAttemptsRequest{AssignmentId: uint64(related.ID)}); err != nil {
				return nil, err
			}
		}
	}
	examReq := &examsClient.UpdatePrivateExamRequest{Id: uint64(assignment.ExamID), Title: strings.TrimSpace(req.Title), Description: req.Description, Questions: make([]*examsClient.PrivateExamQuestion, len(req.Questions)), CorrectionOnly: totalAttempts > 0, ActorId: teacherID, Reason: strings.TrimSpace(req.Reason)}
	for i, question := range req.Questions {
		optionIDs := make([]uint64, len(question.OptionIDs))
		for j, id := range question.OptionIDs {
			optionIDs[j] = uint64(id)
		}
		examReq.Questions[i] = &examsClient.PrivateExamQuestion{Id: uint64(question.ID), Content: strings.TrimSpace(question.Content), Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: int32(question.CorrectOptionIndex), ExcludedFromScore: question.ExcludedFromScore, OptionIds: optionIDs}
	}
	exam, err := s.examsClient.UpdatePrivateExam(ctx, examReq)
	if err != nil {
		return nil, err
	}
	finalActive := false
	if totalAttempts == 0 {
		finalActive = true
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.UpdateAssignmentsContentAndActive(ctx, assignment.ExamID, exam.Title, exam.Description, finalActive); err != nil {
			return err
		}
		if totalAttempts == 0 {
			return repo.RestoreAssignmentActiveStates(ctx, activeStates)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	response := &dto.EditableAssignmentResponse{ID: assignment.ID, ExamID: assignment.ExamID, Title: exam.Title, Description: exam.Description, Questions: make([]*dto.EditableAssignmentQuestion, len(exam.Questions))}
	for i, question := range exam.Questions {
		optionIDs := make([]uint, len(question.OptionIds))
		for j, id := range question.OptionIds {
			optionIDs[j] = uint(id)
		}
		response.Questions[i] = &dto.EditableAssignmentQuestion{ID: uint(question.Id), Content: question.Content, Explanation: question.Explanation, Options: question.Options, CorrectOptionIndex: int(question.CorrectOptionIndex), ExcludedFromScore: question.ExcludedFromScore, OptionIDs: optionIDs}
	}
	return response, nil
}

func (s *service) GetAssignments(ctx context.Context, userID string, classroomID uint, query dto.AssignmentsQuery) ([]*dto.AssignmentResponse, int, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, 0, err
	}

	isTeacher := classroom.TeacherID == userID
	if !isTeacher {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, 0, err
		}
		if !approved {
			return nil, 0, ErrClassroomForbidden
		}
	}

	// Inactive assignments intentionally remain visible so students can review
	// previous results; the response state prevents starting a new attempt.
	assignments, total, err := s.repo.GetAssignmentsByClassroom(ctx, classroomID, query.PaginationQuery)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]string, 0)
	seenUserIDs := make(map[string]struct{})
	if isTeacher {
		for _, assignment := range assignments {
			for _, submission := range assignment.Submissions {
				if _, ok := seenUserIDs[submission.StudentID]; !ok {
					seenUserIDs[submission.StudentID] = struct{}{}
					userIDs = append(userIDs, submission.StudentID)
				}
			}
		}
	} else {
		userIDs = append(userIDs, userID)
	}

	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, 0, err
		}
		users = usersResp.Users
	}

	currentUserID := userID
	if isTeacher {
		currentUserID = ""
	}
	responses := dto.ToAssignmentResponses(assignments, time.Now(), currentUserID, isTeacher, users)
	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetAssignment(ctx context.Context, userID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}

	isTeacher := classroom.TeacherID == userID
	if !isTeacher {
		approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, userID)
		if err != nil {
			return nil, err
		}
		if !approved {
			return nil, ErrClassroomForbidden
		}
	}

	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]string, 0)
	seenUserIDs := make(map[string]struct{})
	if isTeacher {
		for _, submission := range assignment.Submissions {
			if _, ok := seenUserIDs[submission.StudentID]; !ok {
				seenUserIDs[submission.StudentID] = struct{}{}
				userIDs = append(userIDs, submission.StudentID)
			}
		}
	} else {
		userIDs = append(userIDs, userID)
	}

	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, err
		}
		users = usersResp.Users
	}

	currentUserID := userID
	if isTeacher {
		currentUserID = ""
	}
	return dto.ToAssignmentResponse(assignment, time.Now(), currentUserID, isTeacher, users), nil
}

func (s *service) GetGradebook(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}

	assignment, err := s.repo.GetAssignmentByClassroom(ctx, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.GetApprovedMembersByClassroom(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(members))
	seenUserIDs := make(map[string]struct{}, len(members))
	for _, member := range members {
		if _, ok := seenUserIDs[member.UserID]; !ok {
			seenUserIDs[member.UserID] = struct{}{}
			userIDs = append(userIDs, member.UserID)
		}
	}
	for _, submission := range assignment.Submissions {
		if _, ok := seenUserIDs[submission.StudentID]; !ok {
			seenUserIDs[submission.StudentID] = struct{}{}
			userIDs = append(userIDs, submission.StudentID)
		}
	}

	var users []*usersClient.UserInfo
	if len(userIDs) > 0 {
		usersResp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: userIDs})
		if err != nil {
			return nil, err
		}
		users = usersResp.Users
	}

	now := time.Now()
	response := dto.ToAssignmentResponse(assignment, now, "", true, users)
	usersByID := dto.ToUserInfoMap(users)
	submittedStudents := make(map[string]struct{}, len(response.Submissions))
	for _, submission := range response.Submissions {
		if submission.Student != nil {
			submittedStudents[submission.Student.ID] = struct{}{}
		}
	}
	for _, member := range members {
		if _, ok := submittedStudents[member.UserID]; ok {
			continue
		}
		response.Submissions = append(response.Submissions, &dto.AssignmentSubmissionResponse{
			AssignmentID: response.ID,
			Student:      usersByID[member.UserID],
			Status:       model.Status(nil, now, response.DueAt),
		})
	}
	return response, nil
}

func (s *service) GetClassGradebook(ctx context.Context, teacherID string, classroomID uint, query dto.GradebookRangeQuery) (*dto.ClassGradebookResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != teacherID {
		return nil, ErrClassroomForbidden
	}
	assignments, err := s.repo.GetAssignmentsForGradebook(ctx, classroomID, query.From, query.To)
	if err != nil {
		return nil, err
	}
	members, err := s.repo.GetApprovedMembersByClassroom(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(members))
	for i, member := range members {
		ids[i] = member.UserID
	}
	users := []*usersClient.UserInfo{}
	if len(ids) > 0 {
		resp, err := s.usersClient.GetUsersInfo(ctx, &usersClient.GetUsersInfoRequest{Ids: ids})
		if err != nil {
			return nil, err
		}
		users = resp.Users
	}
	usersByID := dto.ToUserInfoMap(users)
	now := time.Now()
	response := &dto.ClassGradebookResponse{ClassroomID: classroomID, Assignments: make([]*dto.GradebookAssignment, len(assignments)), Students: make([]*dto.StudentProgressResponse, 0, len(members))}
	for i, assignment := range assignments {
		response.Assignments[i] = &dto.GradebookAssignment{ID: assignment.ID, Title: assignment.Title, DueAt: assignment.DueAt}
	}
	var totalScore float64
	var scored, completed, total int
	for _, member := range members {
		student := usersByID[member.UserID]
		if student == nil {
			student = &dto.UserInfo{ID: member.UserID}
		}
		progress := &dto.StudentProgressResponse{Student: student, Assignments: make([]*dto.StudentAssignmentProgress, 0, len(assignments))}
		var studentScore float64
		var studentScored, studentCompleted int
		for _, assignment := range assignments {
			var found *model.AssignmentSubmission
			for i := range assignment.Submissions {
				if assignment.Submissions[i].StudentID == member.UserID {
					found = &assignment.Submissions[i]
					break
				}
			}
			status := model.Status(found, now, assignment.DueAt)
			cell := &dto.StudentAssignmentProgress{AssignmentID: assignment.ID, Title: assignment.Title, DueAt: assignment.DueAt, Status: status}
			if found != nil {
				cell.Score = found.Score
			}
			progress.Assignments = append(progress.Assignments, cell)
			total++
			if status == model.SubmissionStatusSubmitted {
				completed++
				studentCompleted++
			}
			if cell.Score != nil {
				totalScore += *cell.Score
				scored++
				studentScore += *cell.Score
				studentScored++
			}
		}
		if studentScored > 0 {
			value := valuex.RoundFloat(studentScore/float64(studentScored), 2)
			progress.AverageScore = &value
		}
		if len(assignments) > 0 {
			progress.CompletionRate = valuex.RoundFloat(float64(studentCompleted)*100/float64(len(assignments)), 2)
		}
		response.Students = append(response.Students, progress)
	}
	if scored > 0 {
		value := valuex.RoundFloat(totalScore/float64(scored), 2)
		response.AverageScore = &value
	}
	if total > 0 {
		response.CompletionRate = valuex.RoundFloat(float64(completed)*100/float64(total), 2)
	}
	return response, nil
}

func (s *service) GetStudentProgress(ctx context.Context, userID string, classroomID uint, studentID string) (*dto.StudentProgressResponse, error) {
	classroom, err := s.repo.GetClassroomByID(ctx, classroomID)
	if err != nil {
		return nil, err
	}
	if classroom.TeacherID != userID && userID != studentID {
		return nil, ErrClassroomForbidden
	}
	approved, err := s.repo.IsStudentApprovedInClassroom(ctx, classroomID, studentID)
	if err != nil {
		return nil, err
	}
	if !approved {
		return nil, ErrClassroomForbidden
	}
	book, err := s.GetClassGradebook(ctx, classroom.TeacherID, classroomID, dto.GradebookRangeQuery{})
	if err != nil {
		return nil, err
	}
	for _, progress := range book.Students {
		if progress.Student != nil && progress.Student.ID == studentID {
			return progress, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *service) GetAssignmentAnalytics(ctx context.Context, teacherID string, classroomID uint, assignmentID uint) (*dto.AssignmentAnalyticsResponse, error) {
	gradebook, err := s.GetGradebook(ctx, teacherID, classroomID, assignmentID)
	if err != nil {
		return nil, err
	}
	response := &dto.AssignmentAnalyticsResponse{AssignmentID: assignmentID, TotalStudents: len(gradebook.Submissions)}
	var sum float64
	var scored int
	for _, submission := range gradebook.Submissions {
		switch submission.Status {
		case model.SubmissionStatusNotStarted:
			response.NotStarted++
		case model.SubmissionStatusInProgress:
			response.InProgress++
		case model.SubmissionStatusSubmitted:
			response.Submitted++
		case model.SubmissionStatusLate:
			response.Late++
		}
		if submission.Score != nil {
			score := *submission.Score
			sum += score
			scored++
			if response.MinimumScore == nil || score < *response.MinimumScore {
				value := score
				response.MinimumScore = &value
			}
			if response.MaximumScore == nil || score > *response.MaximumScore {
				value := score
				response.MaximumScore = &value
			}
		}
	}
	if scored > 0 {
		value := valuex.RoundFloat(sum/float64(scored), 2)
		response.AverageScore = &value
	}
	if response.TotalStudents > 0 {
		response.CompletionRate = valuex.RoundFloat(float64(response.Submitted)*100/float64(response.TotalStudents), 2)
	}
	analytics, err := s.examsClient.GetAssignmentQuestionAnalytics(ctx, &examsClient.GetAssignmentQuestionAnalyticsRequest{AssignmentId: uint64(assignmentID), ExamId: uint64(gradebook.ExamID)})
	if err != nil {
		return nil, err
	}
	for _, question := range analytics.Questions {
		item := &dto.QuestionAnalyticsResponse{QuestionID: uint(question.QuestionId), Content: question.Content, Order: int(question.Order), AnsweredCount: question.AnsweredCount, WrongCount: question.WrongCount}
		if item.AnsweredCount > 0 {
			item.WrongRate = valuex.RoundFloat(float64(item.WrongCount)*100/float64(item.AnsweredCount), 2)
		}
		response.Questions = append(response.Questions, item)
	}
	return response, nil
}
