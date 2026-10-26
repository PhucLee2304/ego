package repository

import (
	"context"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/model"

	"gorm.io/gorm/clause"
)

func (r *repository) CreateAssignments(ctx context.Context, assignments []*model.Assignment) error {
	if len(assignments) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&assignments).Error
}

func (r *repository) GetAssignmentsByExamID(ctx context.Context, examID uint) ([]*model.Assignment, error) {
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

func (r *repository) AssignmentExistsByClassroomAndExam(ctx context.Context, classroomID uint, examID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Assignment{}).
		Where("classroom_id = ? AND exam_id = ?", classroomID, examID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) GetAssignmentsByClassroom(ctx context.Context, classroomID uint, pagination httpx.PaginationQuery) ([]*model.Assignment, int64, error) {
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

func (r *repository) GetAssignmentsDueInRange(ctx context.Context, classroomID uint, from time.Time, to time.Time) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	if err := r.db.WithContext(ctx).
		Where("classroom_id = ? AND due_at >= ? AND due_at < ?", classroomID, from, to).
		Order("due_at ASC, id ASC").
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *repository) GetAssignmentByClassroom(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Submissions").
		Where("classroom_id = ? AND id = ?", classroomID, assignmentID).
		First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *repository) GetAssignmentByClassroomForUpdate(ctx context.Context, classroomID uint, assignmentID uint) (*model.Assignment, error) {
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

func (r *repository) UpdateAssignment(ctx context.Context, assignment *model.Assignment) error {
	return r.db.WithContext(ctx).Save(assignment).Error
}

func (r *repository) UpdateAssignmentsContentAndActive(ctx context.Context, examID uint, title string, description *string, active bool) error {
	return r.db.WithContext(ctx).Model(&model.Assignment{}).Where("exam_id = ?", examID).Updates(map[string]any{
		"title": title, "description": description, "active": active,
	}).Error
}

func (r *repository) RestoreAssignmentActiveStates(ctx context.Context, states map[uint]bool) error {
	for id, active := range states {
		if err := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("id = ?", id).Update("active", active).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *repository) GetAssignmentForAttemptValidation(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Preload("Classroom").
		Where("id = ?", id).
		First(&assignment).Error; err != nil {
		return nil, err
	}

	return &assignment, nil
}

func (r *repository) GetAssignmentForAttemptValidationForUpdate(ctx context.Context, id uint) (*model.Assignment, error) {
	var assignment model.Assignment
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Classroom").Where("id = ?", id).First(&assignment).Error; err != nil {
		return nil, err
	}
	return &assignment, nil
}

func (r *repository) GetAssignmentIDsByClassroom(ctx context.Context, classroomID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.WithContext(ctx).Model(&model.Assignment{}).Where("classroom_id = ?", classroomID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
