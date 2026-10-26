package repository

import (
	"context"
	"time"

	"ego/services/learning/internal/model"
)

// AssignmentQuestionAnalytics là số lượt trả lời / trả lời sai của một câu trong
// các bài đã nộp của assignment.
type AssignmentQuestionAnalytics struct {
	QuestionID    uint
	Content       string
	Order         int
	AnsweredCount int64
	WrongCount    int64
}

func (r *repository) GetAssignmentsForGradebook(ctx context.Context, classroomID uint, from *time.Time, to *time.Time) ([]*model.Assignment, error) {
	var assignments []*model.Assignment
	db := r.db.WithContext(ctx).Where("classroom_id = ?", classroomID)
	if from != nil {
		db = db.Where("due_at >= ?", *from)
	}
	if to != nil {
		db = db.Where("due_at < ?", *to)
	}
	err := db.Preload("Submissions").Order("due_at ASC, id ASC").Find(&assignments).Error
	return assignments, err
}

func (r *repository) GetAssignmentQuestionAnalytics(ctx context.Context, assignmentID uint, examID uint) ([]*AssignmentQuestionAnalytics, error) {
	var results []*AssignmentQuestionAnalytics
	err := r.db.WithContext(ctx).Raw(`
		SELECT q.id AS question_id, q.content, q."order",
		       COUNT(aa.id) FILTER (WHERE aa.selected_option_id IS NOT NULL) AS answered_count,
		       COUNT(aa.id) FILTER (WHERE aa.selected_option_id IS NOT NULL AND aa.is_correct = false) AS wrong_count
		FROM questions q
		JOIN sections s ON s.id = q.section_id AND s.deleted_at IS NULL
		LEFT JOIN attempts a ON a.exam_id = s.exam_id AND a.context_type = ? AND a.context_id = ? AND a.status = ? AND a.deleted_at IS NULL
		LEFT JOIN attempt_answers aa ON aa.attempt_id = a.id AND aa.question_id = q.id AND aa.deleted_at IS NULL
		WHERE s.exam_id = ? AND q.deleted_at IS NULL
		GROUP BY q.id, q.content, q."order"
		ORDER BY q."order" ASC, q.id ASC`, model.AttemptContextTypeClassroomAssignment, assignmentID, model.AttemptStatusSubmitted, examID).
		Scan(&results).Error
	return results, err
}
