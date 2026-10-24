package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
	// Exam
	GetListExam(ctx context.Context, examType model.ExamType, limit, offset int32) ([]*model.Exam, int64, error)
	GetExamByID(ctx context.Context, id uint) (*model.Exam, error)
	CreateExam(ctx context.Context, exam *model.Exam) error
	GetQuestionsByExamID(ctx context.Context, id uint) (*model.Exam, error)

	// Private exam
	GetPrivateAssignmentExam(ctx context.Context, id uint) (*model.Exam, error)
	ReplacePrivateExamQuestions(ctx context.Context, exam *model.Exam, questions []model.Question) error
	CreateExamAudit(ctx context.Context, audit *model.ExamAudit) error
	GetExamAudits(ctx context.Context, examID uint) ([]*model.ExamAudit, error)
	UpdateQuestionScoring(ctx context.Context, questionID uint, excluded bool, correctOptionID uint) error
	GetSubmittedAttemptsByExamID(ctx context.Context, examID uint) ([]*model.Attempt, error)

	// Attempt
	GetActiveAttemptByUserID(ctx context.Context, userID string) (*model.Attempt, error)
	GetAttemptsByUserIDAndStatus(ctx context.Context, userID string, status model.AttemptStatus, limit, offset int32) ([]*model.Attempt, int64, error)
	GetAttemptQuestionsByIDAndUserID(ctx context.Context, id uint, userID string) (*model.Attempt, error)
	GetAttemptForLifecycle(ctx context.Context, id uint, userID string) (*model.Attempt, error)
	CreateAttempt(ctx context.Context, attempt *model.Attempt) error
	UpdateAttempt(ctx context.Context, attempt *model.Attempt) error
	GetExpiredActiveAttempts(ctx context.Context, now time.Time, limit int) ([]*model.Attempt, error)

	// Answer
	GetAttemptForAnswerUpdate(ctx context.Context, id uint, userID string) (*model.Attempt, error)
	UpsertAttemptAnswer(ctx context.Context, answer *model.AttemptAnswer) error
	DeleteAttemptAnswer(ctx context.Context, attemptID, questionID uint) error
	ReplaceAttemptAnswers(ctx context.Context, attemptID uint, answers []model.AttemptAnswer) error

	// History
	GetAttemptHistoryByIDAndUserID(ctx context.Context, id uint, userID string) (*model.History, error)
	UpsertHistory(ctx context.Context, history *model.History) error

	// Classroom
	CreateClassroom(ctx context.Context, classroom *model.Classroom) error
	GetClassrooms(ctx context.Context, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error)
	GetClassroomsByUser(ctx context.Context, userID string, query dto.ClassroomsQuery, teacherIDs []string) ([]*model.Classroom, int64, error)
	GetClassroomByID(ctx context.Context, id uint) (*model.Classroom, error)
	GetClassroomByCode(ctx context.Context, code string) (*model.Classroom, error)
	UpdateClassroom(ctx context.Context, classroom *model.Classroom) error

	// Member
	CountApprovedMembers(ctx context.Context, classroomID uint) (int64, error)
	UpsertMember(ctx context.Context, member *model.Member) error
	GetMember(ctx context.Context, classroomID uint, userID string) (*model.Member, error)
	GetJoinRequests(ctx context.Context, classroomID uint) ([]*model.Member, error)
	DeleteMember(ctx context.Context, member *model.Member) error
	IsStudentApprovedInClassroom(ctx context.Context, classroomID uint, userID string) (bool, error)
	GetApprovedMembersByClassroom(ctx context.Context, classroomID uint) ([]*model.Member, error)

	// Schedule
	CreateSchedule(ctx context.Context, schedule *model.Schedule) error
	GetScheduleByID(ctx context.Context, id uint) (*model.Schedule, error)
	GetSchedulesByRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Schedule, error)
	UpdateSchedule(ctx context.Context, schedule *model.Schedule) error
	DeleteSchedule(ctx context.Context, schedule *model.Schedule) error

	// Room
	CreateRoom(ctx context.Context, room *model.Room) error
	GetRoomByID(ctx context.Context, id uint) (*model.Room, error)
	GetRoomByIDForUpdate(ctx context.Context, id uint) (*model.Room, error)
	GetRoomByName(ctx context.Context, name string) (*model.Room, error)
	SetRoomHostAbsentSince(ctx context.Context, roomID uint, absentSince *time.Time) error
	UpdateRoom(ctx context.Context, room *model.Room) error
	DeleteRoom(ctx context.Context, room *model.Room) error
	HasLiveRoomByClassroom(ctx context.Context, classroomID uint) (bool, error)
	GetLiveRoomsByClassroom(ctx context.Context, classroomID uint) ([]*model.Room, error)
	GetRoomsForLifecycle(ctx context.Context, now time.Time) ([]*model.Room, error)

	// Participant
	GetOrCreateRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error)
	FindRoomParticipantState(ctx context.Context, roomID uint, userID string) (*model.RoomParticipantState, error)
	UpdateRoomParticipantState(ctx context.Context, state *model.RoomParticipantState) error
	LowerRoomHands(ctx context.Context, roomID uint, userID *string) ([]string, error)
	GetRoomParticipantStates(ctx context.Context, roomID uint) ([]*model.RoomParticipantState, error)
	DeleteRoomParticipantStates(ctx context.Context, roomID uint) error
	CreateRoomModerationAudit(ctx context.Context, audit *model.RoomModerationAudit) error
	GetRoomModerationAudits(ctx context.Context, roomID uint) ([]*model.RoomModerationAudit, error)
	OpenParticipantSession(ctx context.Context, roomID uint, userID string, role model.RoomParticipantRole, joinedAt time.Time) error
	CloseParticipantSession(ctx context.Context, roomID uint, userID string, leftAt time.Time) error
	CloseAllParticipantSessions(ctx context.Context, roomID uint, leftAt time.Time) error
	GetOpenParticipantSessions(ctx context.Context, roomID uint) ([]*model.RoomParticipant, error)
	GetRoomAttendance(ctx context.Context, roomID uint, userID *string) ([]*model.RoomParticipant, error)

	// Assignment
	CreateAssignments(ctx context.Context, assignments []*model.Assignment) error
	GetAssignmentsByExamID(ctx context.Context, examID uint) ([]*model.Assignment, error)
	AssignmentExistsByClassroomAndExam(ctx context.Context, classroomID uint, examID uint) (bool, error)
	GetAssignmentsByClassroom(ctx context.Context, classroomID uint, pagination httpx.PaginationQuery) ([]*model.Assignment, int64, error)
	GetAssignmentsDueInRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Assignment, error)
	GetAssignmentByClassroom(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error)
	GetAssignmentByClassroomForUpdate(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error)
	UpdateAssignment(ctx context.Context, assignment *model.Assignment) error
	UpdateAssignmentsContentAndActive(ctx context.Context, examID uint, title string, description *string, active bool) error
	RestoreAssignmentActiveStates(ctx context.Context, states map[uint]bool) error

	// Assignment attempt
	GetActiveAttemptsByAssignmentID(ctx context.Context, assignmentID uint) ([]*model.Attempt, error)
	ExpireActiveAssignmentAttempts(ctx context.Context, assignmentID uint, now time.Time) error
	CountAssignmentAttempts(ctx context.Context, assignmentID uint) (int64, int64, error)
	UpdateActiveAssignmentAttemptSchedule(ctx context.Context, assignmentID uint, duration int, dueAt time.Time) (int64, error)
	ResetAssignmentAttempt(ctx context.Context, assignmentID uint, studentID string) error

	// Submission
	GetAssignmentSubmissionByAssignmentAndStudent(ctx context.Context, assignmentID uint, studentID string) (*model.AssignmentSubmission, error)
	DeleteAssignmentSubmission(ctx context.Context, assignmentID uint, studentID string) error

	// Notification outbox
	CreateNotificationOutbox(ctx context.Context, event *model.NotificationOutbox) error
	ClaimNotificationOutbox(ctx context.Context, now time.Time, limit int) ([]*model.NotificationOutbox, error)
	MarkNotificationOutboxProcessed(ctx context.Context, id uint, at time.Time) error
	MarkNotificationOutboxFailed(ctx context.Context, id uint, attempts int, next time.Time, message string) error
	CancelNotificationOutbox(ctx context.Context, eventKeyPrefix string) error

	Transaction(ctx context.Context, fn func(Repository) error) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Transaction(ctx context.Context, fn func(Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(NewRepository(tx))
	})
}
