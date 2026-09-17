package repository

import (
	"context"
	platformdb "ego/platform/db"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"time"

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

func (r *Repository) GetListExam(ctx context.Context, examType model.ExamType, limit, offset int32) ([]*model.Exam, int64, error) {
	var exams []*model.Exam
	var total int64
	query := r.db.WithContext(ctx).Model(&model.Exam{}).Where("type = ?", examType)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.
		Order("id ASC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&exams).Error; err != nil {
		return nil, 0, err
	}

	return exams, total, nil
}

func (r *Repository) GetExamByID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *Repository) GetQuestionsByExamID(ctx context.Context, id uint) (*model.Exam, error) {
	var exam model.Exam
	if err := r.db.WithContext(ctx).
		Preload("Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		First(&exam, id).Error; err != nil {
		return nil, err
	}

	return &exam, nil
}

func (r *Repository) GetActiveAttemptByUserID(ctx context.Context, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.AttemptStatusInProgress).
		Order("id DESC").
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) GetAttemptsByUserIDAndStatus(ctx context.Context, userID string, status model.AttemptStatus, limit, offset int32) ([]*model.Attempt, int64, error) {
	var attempts []*model.Attempt
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Attempt{}).
		Where("user_id = ? AND status = ?", userID, status)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.WithContext(ctx).
		Preload("Exam").
		Preload("Answers").
		Preload("History").
		Where("user_id = ? AND status = ?", userID, status).
		Order("id DESC").
		Limit(int(limit)).
		Offset(int(offset)).
		Find(&attempts).Error; err != nil {
		return nil, 0, err
	}

	return attempts, total, nil
}

func (r *Repository) GetAttemptQuestionsByIDAndUserID(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Preload("Answers").
		Preload("Exam.Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Where("id = ? AND user_id = ?", id, userID).
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) GetAttemptForAnswerUpdate(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	if err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Exam.Sections.Questions.Options").
		Where("id = ? AND user_id = ?", id, userID).
		First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) GetAttemptForLifecycle(ctx context.Context, id uint, userID string) (*model.Attempt, error) {
	var attempt model.Attempt
	query := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Answers").
		Preload("History").
		Preload("Exam.Sections", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Groups.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Preload("Exam.Sections.Questions.Options", func(db *gorm.DB) *gorm.DB {
			return db.Order(clause.OrderByColumn{Column: clause.Column{Name: "order"}, Desc: false})
		}).
		Where("id = ? AND user_id = ?", id, userID)
	if err := query.First(&attempt).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
}

func (r *Repository) GetAttemptHistoryByIDAndUserID(ctx context.Context, id uint, userID string) (*model.History, error) {
	var history model.History
	if err := r.db.WithContext(ctx).
		Joins("JOIN attempts ON attempts.id = histories.attempt_id AND attempts.deleted_at IS NULL").
		Where("histories.attempt_id = ? AND attempts.user_id = ?", id, userID).
		First(&history).Error; err != nil {
		return nil, err
	}

	return &history, nil
}

func (r *Repository) GetExpiredActiveAttempts(ctx context.Context, now time.Time, limit int) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Select("id", "user_id").
		Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ?", model.AttemptStatusInProgress, now).
		Order("expires_at ASC").
		Limit(limit).
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (r *Repository) CreateAttempt(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).Create(attempt).Error
}

func (r *Repository) UpsertAttemptAnswer(ctx context.Context, answer *model.AttemptAnswer) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "attempt_id"},
			{Name: "question_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"selected_option_id",
			"is_correct",
			"answered_at",
			"updated_at",
			"deleted_at",
		}),
	}).Create(answer).Error
}

func (r *Repository) DeleteAttemptAnswer(ctx context.Context, attemptID, questionID uint) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
		Delete(&model.AttemptAnswer{}).Error
}

func (r *Repository) ReplaceAttemptAnswers(ctx context.Context, attemptID uint, answers []model.AttemptAnswer) error {
	if err := r.db.WithContext(ctx).
		Unscoped().
		Where("attempt_id = ?", attemptID).
		Delete(&model.AttemptAnswer{}).Error; err != nil {
		return err
	}

	if len(answers) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&answers).Error
}

func (r *Repository) UpdateAttempt(ctx context.Context, attempt *model.Attempt) error {
	return r.db.WithContext(ctx).
		Model(attempt).
		Select("status", "submitted_at", "correct_answers", "score", "updated_at").
		Updates(attempt).Error
}

func (r *Repository) UpsertHistory(ctx context.Context, history *model.History) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "attempt_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"data",
			"exam_title",
			"exam_type",
			"exam_year",
			"updated_at",
			"deleted_at",
		}),
	}).Create(history).Error
}

func (r *Repository) CreateOutboxEvent(ctx context.Context, event *model.OutboxEvent) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
}

func (r *Repository) ClaimPendingOutboxEvents(ctx context.Context, now time.Time, limit int, visibilityTimeout time.Duration) ([]*model.OutboxEvent, error) {
	var events []*model.OutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND status IN ? AND next_attempt_at <= ?", []model.OutboxEventStatus{
				model.OutboxEventStatusPending,
				model.OutboxEventStatusProcessing,
				model.OutboxEventStatusFailed,
			}, now).
			Order("next_attempt_at ASC, id ASC").
			Limit(limit).
			Find(&events).Error; err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(events))
		for _, event := range events {
			ids = append(ids, event.ID)
		}

		return tx.Model(&model.OutboxEvent{}).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":          model.OutboxEventStatusProcessing,
				"next_attempt_at": now.Add(visibilityTimeout),
			}).Error
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (r *Repository) MarkOutboxEventProcessed(ctx context.Context, event *model.OutboxEvent, processedAt time.Time) error {
	return r.db.WithContext(ctx).
		Model(event).
		Updates(map[string]any{
			"processed_at": processedAt,
			"status":       model.OutboxEventStatusProcessed,
			"last_error":   nil,
			"updated_at":   processedAt,
		}).Error
}

func (r *Repository) MarkOutboxEventFailed(ctx context.Context, event *model.OutboxEvent, attempts int, nextAttemptAt time.Time, lastError string, permanent bool) error {
	status := model.OutboxEventStatusFailed
	if permanent {
		status = model.OutboxEventStatusFailedPermanent
	}

	return r.db.WithContext(ctx).
		Model(event).
		Updates(map[string]any{
			"attempts":        attempts,
			"status":          status,
			"next_attempt_at": nextAttemptAt,
			"last_error":      lastError,
			"updated_at":      time.Now(),
		}).Error
}

func (r *Repository) GetPendingOutboxEvents(ctx context.Context, filter dto.GetPendingOutboxEventsQuery) ([]*model.OutboxEvent, int64, error) {
	var events []*model.OutboxEvent
	var total int64
	query := r.db.WithContext(ctx).
		Model(&model.OutboxEvent{}).
		Where("processed_at IS NULL AND status <> ?", model.OutboxEventStatusProcessed)
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.MinAttempts > 0 {
		query = query.Where("attempts >= ?", filter.MinAttempts)
	}
	if filter.HasError != nil {
		if *filter.HasError {
			query = query.Where("last_error IS NOT NULL AND last_error <> ''")
		} else {
			query = query.Where("last_error IS NULL OR last_error = ''")
		}
	}
	if filter.CreatedFrom != nil {
		query = query.Where("created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("created_at <= ?", *filter.CreatedTo)
	}
	if filter.NextAttemptFrom != nil {
		query = query.Where("next_attempt_at >= ?", *filter.NextAttemptFrom)
	}
	if filter.NextAttemptTo != nil {
		query = query.Where("next_attempt_at <= ?", *filter.NextAttemptTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.
		Order("attempts DESC, next_attempt_at ASC, id ASC").
		Limit(int(filter.Limit())).
		Offset(int(filter.Offset())).
		Find(&events).Error; err != nil {
		return nil, 0, err
	}

	return events, total, nil
}

func (r *Repository) GetSubmittedClassroomAssignmentAttemptsForReconcile(ctx context.Context, limit int) ([]*model.Attempt, error) {
	var attempts []*model.Attempt
	if err := r.db.WithContext(ctx).
		Where("status = ? AND context_type = ? AND context_id IS NOT NULL AND score IS NOT NULL",
			model.AttemptStatusSubmitted,
			model.AttemptContextTypeClassroomAssignment,
		).
		Order("submitted_at DESC, id DESC").
		Limit(limit).
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}
