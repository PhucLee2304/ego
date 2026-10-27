package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md). Mọi luồng sửa attempt khóa dòng
// attempt trước, rồi mới tới submission và event (mục 1.6).

// GetAttemptForIntegrity khóa attempt (FOR UPDATE), không preload.
func (r *repository) GetAttemptForIntegrity(ctx context.Context, id uint) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).
		First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

// GetAssignmentAttemptForIntegrity khóa attempt của học sinh trong một assignment.
func (r *repository) GetAssignmentAttemptForIntegrity(ctx context.Context, assignmentID uint, studentID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("context_type = ? AND context_id = ? AND user_id = ?", model.AttemptContextTypeClassroomAssignment, assignmentID, studentID).
		First(&attempt).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (r *repository) UpdateAttemptSession(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).
		Model(attempt).
		Select("session_token_hash", "session_epoch", "session_last_seen_at", "grace_until", "updated_at").
		Updates(attempt).Error
}

func (r *repository) UpdateAttemptIntegrity(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).
		Model(attempt).
		Select("violation_count", "correct_answers", "raw_score", "score", "integrity_outcome",
			"reinstated_at", "reinstated_by", "reinstate_note", "updated_at").
		Updates(attempt).Error
}

// TouchAttemptSession ghi lần cuối thấy kết nối chủ phiên (khi kết nối ngắt).
func (r *repository) TouchAttemptSession(ctx context.Context, attemptID uint, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.Attempt{}).
		Where("id = ? AND status = ?", attemptID, model.AttemptStatusInProgress).
		Update("session_last_seen_at", at).Error
}

func (r *repository) CreateIntegrityEvent(ctx context.Context, event *model.AttemptIntegrityEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *repository) GetIntegrityEvent(ctx context.Context, attemptID uint, eventID string) (*model.AttemptIntegrityEvent, error) {
	var event model.AttemptIntegrityEvent
	if err := r.db.WithContext(ctx).Where("attempt_id = ? AND event_id = ?", attemptID, eventID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

// HasIntegrityViolationForEpisode: episode (startEventID) đã được tính vi phạm chưa.
func (r *repository) HasIntegrityViolationForEpisode(ctx context.Context, attemptID uint, startEventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.AttemptIntegrityEvent{}).
		Where("attempt_id = ? AND start_event_id = ? AND level = ?", attemptID, startEventID, model.IntegrityLevelViolation).
		Count(&count).Error
	return count > 0, err
}

// GetAssignmentIntegrityEvent tìm event theo eventId trong các attempt của assignment.
func (r *repository) GetAssignmentIntegrityEvent(ctx context.Context, assignmentID uint, eventID string) (*model.AttemptIntegrityEventRow, error) {
	var row model.AttemptIntegrityEventRow
	if err := r.assignmentIntegrityEvents(ctx, assignmentID).
		Where("attempt_integrity_events.event_id = ?", eventID).
		Take(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// MarkIntegrityEventReviewed chỉ ghi khi event chưa được review; false nếu đã có review.
func (r *repository) MarkIntegrityEventReviewed(ctx context.Context, id uint, review model.IntegrityReview, by string, at time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&model.AttemptIntegrityEvent{}).
		Where("id = ? AND reviewed_at IS NULL", id).
		Updates(map[string]any{"review": review, "reviewed_by": by, "reviewed_at": at})
	return result.RowsAffected == 1, result.Error
}

type IntegrityEventFilter struct {
	StudentID string
	Pending   bool
	Level     string
}

func (r *repository) GetAssignmentIntegrityEvents(ctx context.Context, assignmentID uint, filter IntegrityEventFilter, limit, offset int32) ([]*model.AttemptIntegrityEventRow, int64, error) {
	query := r.assignmentIntegrityEvents(ctx, assignmentID)
	if filter.StudentID != "" {
		query = query.Where("attempts.user_id = ?", filter.StudentID)
	}
	if filter.Pending {
		query = query.Where("attempt_integrity_events.level = ? AND attempt_integrity_events.review IS NULL", model.IntegrityLevelViolation)
	}
	if filter.Level != "" {
		query = query.Where("attempt_integrity_events.level = ?", filter.Level)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []*model.AttemptIntegrityEventRow
	if err := query.Order("attempt_integrity_events.received_at DESC, attempt_integrity_events.id DESC").
		Limit(int(limit)).Offset(int(offset)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (r *repository) assignmentIntegrityEvents(ctx context.Context, assignmentID uint) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("attempt_integrity_events").
		Select("attempt_integrity_events.*, attempts.user_id AS student_id").
		Joins("JOIN attempts ON attempts.id = attempt_integrity_events.attempt_id AND attempts.deleted_at IS NULL").
		Where("attempt_integrity_events.deleted_at IS NULL").
		Where("attempts.context_type = ? AND attempts.context_id = ?", model.AttemptContextTypeClassroomAssignment, assignmentID)
}

// GetAssignmentIntegritySummaries gom số vi phạm chờ xử lý theo attempt bằng một query.
func (r *repository) GetAssignmentIntegritySummaries(ctx context.Context, assignmentID uint) ([]*model.AttemptIntegritySummary, error) {
	var rows []*model.AttemptIntegritySummary
	err := r.db.WithContext(ctx).
		Table("attempts").
		Select(`attempts.id AS attempt_id, attempts.proctored, attempts.violation_count,
			attempts.integrity_outcome, attempts.termination_reason,
			COUNT(e.id) FILTER (WHERE e.level = ? AND e.review IS NULL) AS pending_violations`, model.IntegrityLevelViolation).
		Joins("LEFT JOIN attempt_integrity_events e ON e.attempt_id = attempts.id AND e.deleted_at IS NULL").
		Where("attempts.deleted_at IS NULL AND attempts.proctored = true").
		Where("attempts.context_type = ? AND attempts.context_id = ?", model.AttemptContextTypeClassroomAssignment, assignmentID).
		Group("attempts.id").
		Scan(&rows).Error
	return rows, err
}
