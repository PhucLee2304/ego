package repository

import (
	"context"
	"time"

	platformdb "ego/platform/db"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"

	"gorm.io/gorm"
)

type Repository interface {
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
