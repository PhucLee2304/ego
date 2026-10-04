package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/classrooms/internal/model"
	"strings"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Transaction(ctx context.Context, fn func(repo *Repository) error) error {
	return platformdb.WithTx(ctx, r.db, func(tx *gorm.DB) error {
		return fn(&Repository{db: tx})
	})
}

func (r *Repository) CreateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Create(classroom).Error
}

func (r *Repository) GetClassrooms(ctx context.Context, pagination httpx.PaginationQuery, query string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Classroom{})
	query = strings.TrimSpace(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("code ILIKE ? OR name ILIKE ?", like, like)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("updated_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *Repository) GetClassroomsByUser(ctx context.Context, userID string, pagination httpx.PaginationQuery, query string) ([]*model.Classroom, int64, error) {
	var classrooms []*model.Classroom
	var total int64
	db := r.db.WithContext(ctx).
		Model(&model.Classroom{}).
		Where("teacher_id = ? OR EXISTS (?)",
			userID,
			r.db.Model(&model.Member{}).
				Select("1").
				Where("members.classroom_id = classrooms.id").
				Where("members.user_id = ? AND members.approved = ?", userID, true),
		)
	query = strings.TrimSpace(query)
	if query != "" {
		like := "%" + query + "%"
		db = db.Where("code ILIKE ? OR name ILIKE ?", like, like)
	}

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Preload("Members", "approved = ?", true).
		Order("updated_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&classrooms).Error; err != nil {
		return nil, 0, err
	}

	return classrooms, total, nil
}

func (r *Repository) GetClassroomByID(ctx context.Context, id uint) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).
		Preload("Members", "approved = ?", true).
		First(&classroom, id).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *Repository) GetClassroomByCode(ctx context.Context, code string) (*model.Classroom, error) {
	var classroom model.Classroom
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&classroom).Error; err != nil {
		return nil, err
	}
	return &classroom, nil
}

func (r *Repository) UpdateClassroom(ctx context.Context, classroom *model.Classroom) error {
	return r.db.WithContext(ctx).Save(classroom).Error
}

func (r *Repository) CountApprovedMembers(ctx context.Context, classroomID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Joins("JOIN classrooms ON classrooms.id = members.classroom_id").
		Where("members.classroom_id = ? AND members.approved = ?", classroomID, true).
		Where("members.user_id <> classrooms.teacher_id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) UpsertMember(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).
		Where("classroom_id = ? AND user_id = ?", member.ClassroomID, member.UserID).
		Assign(map[string]any{
			"approved":    member.Approved,
			"approved_at": member.ApprovedAt,
			"joined_at":   member.JoinedAt,
		}).
		FirstOrCreate(member).Error
}

func (r *Repository) GetMember(ctx context.Context, classroomID uint, userID string) (*model.Member, error) {
	var member model.Member
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("classroom_id = ? AND user_id = ?", classroomID, userID).
		First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *Repository) GetJoinRequests(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND approved IS NULL", classroomID).
		Preload("Classroom").
		Order("created_at DESC, id DESC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *Repository) DeleteMember(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).Delete(member).Error
}

func (r *Repository) GetAssignmentForAttemptValidation(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("id = ?", id).
		First(&assignment).Error; err != nil {
		return nil, err
	}

	return &assignment, nil
}

func (r *Repository) IsStudentApprovedInClassroom(ctx context.Context, classroomID uint, userID string) (bool, error) {
	var count int64
	approved := true
	if err := r.db.WithContext(ctx).
		Model(&model.Member{}).
		Where("classroom_id = ? AND user_id = ? AND approved = ?", classroomID, userID, approved).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *Repository) UpsertAssignmentSubmission(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) UpsertAssignmentSubmissionResult(ctx context.Context, submission *model.AssignmentSubmission) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", submission.AssignmentID, submission.StudentID).
		Assign(map[string]any{
			"attempt_id": submission.AttemptID,
			"score":      submission.Score,
		}).
		FirstOrCreate(submission).Error
}

func (r *Repository) GetAssignmentSubmissionByAssignmentAndStudent(ctx context.Context, assignmentID uint, studentID string) (*model.AssignmentSubmission, error) {
	var submission model.AssignmentSubmission
	if err := r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		First(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}
