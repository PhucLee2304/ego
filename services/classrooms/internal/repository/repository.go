package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/services/classrooms/internal/model"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
		Order("updated_at DESC, id DESC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *Repository) DeleteMember(ctx context.Context, member *model.Member) error {
	return r.db.WithContext(ctx).Delete(member).Error
}

func (r *Repository) CreateAssignments(ctx context.Context, assignments []*model.Assignment) error {
	if len(assignments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&assignments).Error
}

func (r *Repository) GetAssignmentsByExamID(ctx context.Context, examID uint) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Preload("Submissions").
		Where("exam_id = ?", examID).
		Order("id ASC").
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *Repository) AssignmentExistsByClassroomAndExam(ctx context.Context, classroomID uint, examID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Assignment{}).
		Where("classroom_id = ? AND exam_id = ?", classroomID, examID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) GetAssignmentsByClassroom(ctx context.Context, classroomID uint, pagination httpx.PaginationQuery) ([]*model.Assignment, int64, error) {
	var assignments []*model.Assignment
	var total int64
	db := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("classroom_id = ?", classroomID)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.
		Preload("Submissions").
		Order("opens_at DESC, id DESC").
		Limit(int(pagination.Limit())).
		Offset(int(pagination.Offset())).
		Find(&assignments).Error; err != nil {
		return nil, 0, err
	}
	return assignments, total, nil
}

func (r *Repository) GetAssignmentByClassroom(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Submissions").
		Where("classroom_id = ? AND id = ?", classroomID, assignmentID).
		First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) GetAssignmentByClassroomForUpdate(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Submissions").
		Where("classroom_id = ? AND id = ?", classroomID, assignmentID).
		First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) GetAssignmentForAttemptValidationForUpdate(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Classroom").Where("id = ?", id).First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *Repository) UpdateAssignment(ctx context.Context, assignment *model.Assignment) error {
	return r.db.WithContext(ctx).Save(assignment).Error
}

func (r *Repository) UpdateAssignmentsContentAndActive(ctx context.Context, examID uint, title string, description *string, active bool) error {
	return r.db.WithContext(ctx).Model(&model.Assignment{}).Where("exam_id = ?", examID).Updates(map[string]any{
		"title": title, "description": description, "active": active,
	}).Error
}

func (r *Repository) RestoreAssignmentActiveStates(ctx context.Context, states map[uint]bool) error {
	for id, active := range states {
		if err := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("id = ?", id).Update("active", active).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) GetApprovedMembersByClassroom(ctx context.Context, classroomID uint) ([]*model.Member, error) {
	var members []*model.Member
	if err := r.db.WithContext(ctx).
		Where("classroom_id = ? AND approved = ?", classroomID, true).
		Order("joined_at ASC, id ASC").
		Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
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

func (r *Repository) DeleteAssignmentSubmission(ctx context.Context, assignmentID uint, studentID string) error {
	return r.db.WithContext(ctx).
		Where("assignment_id = ? AND student_id = ?", assignmentID, studentID).
		Delete(&model.AssignmentSubmission{}).Error
}
