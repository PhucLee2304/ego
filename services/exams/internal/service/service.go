package service

import (
	"context"
	classroomsClient "ego/api/gen/go/classrooms"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/platform/valuex"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service interface {
	GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, error)
	GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, error)
	CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, error)
	GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int, error)
	GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, error)
	UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error)
	SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, error)
	CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, error)
	GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, error)
	AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error)
	ProcessOutboxEvents(ctx context.Context, limit int) (int, error)
	GetPendingOutboxEvents(ctx context.Context, query dto.GetPendingOutboxEventsQuery) ([]*dto.OutboxEventResponse, int, error)
	ReconcileClassroomAssignmentSubmissions(ctx context.Context, limit int) (int, error)
}

type service struct {
	repo             *repository.Repository
	classroomsClient classroomsClient.ClassroomServiceClient
}

func New(repo *repository.Repository, classroomsClient classroomsClient.ClassroomServiceClient) Service {
	return &service{
		repo:             repo,
		classroomsClient: classroomsClient,
	}
}

var (
	ErrAttemptNotActive            = errors.New("[ERROR] Attempt is not active")
	ErrAttemptExpired              = errors.New("[ERROR] Attempt has expired")
	ErrAttemptAlreadySubmitted     = errors.New("[CONFLICT] Attempt has already been submitted")
	ErrAttemptCancelled            = errors.New("[CONFLICT] Attempt has been cancelled")
	ErrAssignmentAttemptCancel     = errors.New("[ERROR] Classroom assignment attempts cannot be cancelled; submit the attempt instead")
	ErrQuestionNotBelongToAttempt  = errors.New("[ERROR] Question does not belong to attempt")
	ErrOptionNotBelongToQuestion   = errors.New("[ERROR] Option does not belong to question")
	ErrAttemptHistoryNotFound      = errors.New("[ERROR] Attempt history not found")
	ErrContextAttemptAlreadyExists = errors.New("[CONFLICT] Attempt already exists for this context")
)

const maxAutoSubmitWorkers = 10
const outboxVisibilityTimeout = time.Minute
const maxOutboxAttempts = 10

type classroomAssignmentAttemptCreatedPayload struct {
	StudentID    string `json:"studentId"`
	AssignmentID uint   `json:"assignmentId"`
	AttemptID    uint   `json:"attemptId"`
}

type classroomAssignmentAttemptSubmittedPayload struct {
	StudentID    string  `json:"studentId"`
	AssignmentID uint    `json:"assignmentId"`
	AttemptID    uint    `json:"attemptId"`
	Score        float64 `json:"score"`
}

func (s *service) GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, error) {
	exams, total, err := s.repo.GetListExam(ctx, query.Type, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, err
	}

	examDTOs := make([]*dto.GetExamsResponse, len(exams))
	for i, exam := range exams {
		examDTOs[i] = dto.ToGetExamsResponse(exam)
	}

	return examDTOs, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, error) {
	exam, err := s.repo.GetExamByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return dto.ToGetExamResponse(exam), nil
}

func (s *service) CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, error) {
	if req.ContextType == model.AttemptContextTypeClassroomAssignment {
		_, err := s.classroomsClient.ValidateAssignmentAttempt(ctx, &classroomsClient.ValidateAssignmentAttemptRequest{
			UserId:       userID,
			AssignmentId: uint64(*req.ContextID),
			ExamId:       uint64(examID),
		})
		if err != nil {
			return nil, err
		}
	}

	activeAttempt, activeErr := s.repo.GetActiveAttemptByUserID(ctx, userID)
	if activeErr == nil && activeAttempt.ExpiresAt != nil && !time.Now().Before(*activeAttempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, activeAttempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, err
		}
	} else if activeErr != nil && !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return nil, activeErr
	}

	var attempt *model.Attempt
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if _, err := repo.GetActiveAttemptByUserID(ctx, userID); err == nil {
			return errors.New("[CONFLICT] Active attempt already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		exam, err := repo.GetQuestionsByExamID(ctx, examID)
		if err != nil {
			return err
		}

		if (exam.Type == model.ExamTypeTHPT || exam.Type == model.ExamTypeTOEIC) && !exam.IsPublic {
			return gorm.ErrRecordNotFound
		}

		if err := dto.ValidateCreateExamAttemptRequestForExam(exam.Type, req); err != nil {
			return err
		}

		now := time.Now()
		var duration *int
		if req.Mode == model.AttemptModeTest {
			value, ok := model.GetTestDurationByExamType(exam.Type)
			if !ok {
				return errors.New("[ERROR] Unsupported exam type")
			}
			duration = &value
		} else if req.Duration != nil {
			duration = valuex.MapPtr(req.Duration, func(value model.AttemptDuration) int {
				return int(value)
			})
		}

		var expiresAt *time.Time
		if duration != nil {
			value := now.Add(time.Duration(*duration) * time.Minute)
			expiresAt = &value
		}

		var sectionCode *model.SectionCode
		partCodes := make([]model.PartCode, 0, len(req.Parts))
		if len(req.Parts) > 0 {
			partCodes = append(partCodes, req.Parts...)
		} else if req.Section != nil {
			value := *req.Section
			sectionCode = &value
		} else if exam.Type == model.ExamTypeTHPT && req.Mode == model.AttemptModePractice {
			value := model.SectionCodeFull
			sectionCode = &value
		}

		partSet := make(map[model.PartCode]struct{}, len(partCodes))
		for _, part := range partCodes {
			partSet[part] = struct{}{}
		}

		totalQuestions := 0
		for _, section := range exam.Sections {
			if sectionCode != nil && section.Code != *sectionCode {
				continue
			}

			for _, question := range section.Questions {
				if len(partSet) > 0 && (question.Part == nil || !partInSet(*question.Part, partSet)) {
					continue
				}
				totalQuestions++
			}
		}

		attempt = &model.Attempt{
			Mode:           req.Mode,
			Status:         model.AttemptStatusInProgress,
			SectionCode:    sectionCode,
			PartCodes:      partCodes,
			StartedAt:      now,
			ExpiresAt:      expiresAt,
			SubmittedAt:    nil,
			Duration:       duration,
			TotalQuestions: totalQuestions,
			CorrectAnswers: nil,
			Score:          nil,
			ExamID:         exam.ID,
			UserID:         userID,
			ContextType:    req.ContextType,
			ContextID:      req.ContextID,
		}

		if err := repo.CreateAttempt(ctx, attempt); err != nil {
			return err
		}
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
			payload, err := json.Marshal(classroomAssignmentAttemptCreatedPayload{
				StudentID:    userID,
				AssignmentID: *attempt.ContextID,
				AttemptID:    attempt.ID,
			})
			if err != nil {
				return err
			}
			if err := repo.CreateOutboxEvent(ctx, &model.OutboxEvent{
				Type:          model.OutboxEventTypeClassroomAssignmentAttemptCreated,
				AggregateID:   valuex.UintStringPtr(attempt.ID),
				Status:        model.OutboxEventStatusPending,
				Payload:       payload,
				NextAttemptAt: time.Now(),
			}); err != nil {
				return err
			}
		}
		attempt.Exam = *exam
		attempt.Answers = []model.AttemptAnswer{}

		return nil
	})
	if err != nil {
		if platformdb.IsUniqueViolation(err, model.AttemptActiveUserUniqueIndex) {
			return nil, errors.New("[CONFLICT] Active attempt already exists")
		}
		if platformdb.IsUniqueViolation(err, model.AttemptClassroomAssignmentUniqueIndex) {
			return nil, ErrContextAttemptAlreadyExists
		}
		return nil, err
	}

	return dto.ToAttemptResponseWithQuestions(attempt), nil
}

func (s *service) GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int, error) {
	attempts, total, err := s.repo.GetAttemptsByUserIDAndStatus(ctx, userID, query.Status, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, err
	}

	now := time.Now()
	responses := make([]*dto.AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = dto.ToAttemptResponse(attempt, now)
	}

	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, error) {
	attempt, err := s.repo.GetAttemptQuestionsByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	if attempt.Status != model.AttemptStatusInProgress {
		return nil, errors.New("[ERROR] Attempt is not active")
	}

	if attempt.ExpiresAt != nil && !time.Now().Before(*attempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, attempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, err
		}
		return nil, ErrAttemptExpired
	}

	return dto.ToAttemptResponseWithQuestions(attempt), nil
}

func (s *service) UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error) {
	var response *dto.UpdateAttemptAnswerResponse
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		attempt, err := repo.GetAttemptForAnswerUpdate(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		if attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptNotActive
		}

		now := time.Now()
		if attempt.ExpiresAt != nil && !now.Before(*attempt.ExpiresAt) {
			return ErrAttemptExpired
		}

		var matchedQuestion *model.Question
		var matchedOption *model.Option
		for sectionIdx := range attempt.Exam.Sections {
			section := &attempt.Exam.Sections[sectionIdx]
			if attempt.SectionCode != nil && section.Code != *attempt.SectionCode {
				continue
			}

			for questionIdx := range section.Questions {
				question := &section.Questions[questionIdx]
				if question.ID != questionID {
					continue
				}
				if len(attempt.PartCodes) > 0 && (question.Part == nil || !partInSlice(*question.Part, attempt.PartCodes)) {
					continue
				}
				matchedQuestion = question
				break
			}
			if matchedQuestion != nil {
				break
			}
		}
		if matchedQuestion == nil {
			return ErrQuestionNotBelongToAttempt
		}

		if optionID != nil {
			for optionIdx := range matchedQuestion.Options {
				option := &matchedQuestion.Options[optionIdx]
				if option.ID == *optionID {
					matchedOption = option
					break
				}
			}
			if matchedOption == nil {
				return ErrOptionNotBelongToQuestion
			}
		}

		var isCorrect *bool
		if matchedOption != nil {
			value := matchedOption.IsCorrect
			isCorrect = &value
		}

		if optionID == nil {
			if err := repo.DeleteAttemptAnswer(ctx, attempt.ID, matchedQuestion.ID); err != nil {
				return err
			}
		} else {
			answer := &model.AttemptAnswer{
				AttemptID:        attempt.ID,
				QuestionID:       matchedQuestion.ID,
				SelectedOptionID: optionID,
				IsCorrect:        isCorrect,
				AnsweredAt:       &now,
			}
			if err := repo.UpsertAttemptAnswer(ctx, answer); err != nil {
				return err
			}
		}

		response = &dto.UpdateAttemptAnswerResponse{
			AttemptID:        attempt.ID,
			QuestionID:       matchedQuestion.ID,
			SelectedOptionID: optionID,
			AnsweredAt:       now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *service) SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, error) {
	return s.submitAttempt(ctx, userID, attemptID, &req.Answers)
}

func (s *service) CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, error) {
	attempt, err := s.repo.GetAttemptForLifecycle(ctx, attemptID, userID)
	if err != nil {
		return nil, err
	}
	if attempt.ContextType == model.AttemptContextTypeClassroomAssignment {
		return nil, ErrAssignmentAttemptCancel
	}

	var response *dto.AttemptResponse
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		attempt, err = repo.GetAttemptForLifecycle(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		switch attempt.Status {
		case model.AttemptStatusCancelled:
			if attempt.History == nil {
				return ErrAttemptHistoryNotFound
			}
			response, err = dto.UnmarshalAttemptHistory(attempt.History.Data)
			return err
		case model.AttemptStatusSubmitted:
			return ErrAttemptAlreadySubmitted
		case model.AttemptStatusInProgress:
		default:
			return ErrAttemptNotActive
		}

		attempt.Status = model.AttemptStatusCancelled
		attempt.SubmittedAt = nil
		attempt.CorrectAnswers = nil
		attempt.Score = nil
		if err := repo.UpdateAttempt(ctx, attempt); err != nil {
			return err
		}

		data, err := dto.MarshalAttemptHistory(attempt)
		if err != nil {
			return err
		}
		history := &model.History{
			Data:      datatypes.JSON(data),
			AttemptID: attempt.ID,
			ExamTitle: attempt.Exam.Title,
			ExamType:  attempt.Exam.Type,
			ExamYear:  attempt.Exam.Year,
		}
		if err := repo.UpsertHistory(ctx, history); err != nil {
			return err
		}
		attempt.History = history
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *service) GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, error) {
	history, err := s.repo.GetAttemptHistoryByIDAndUserID(ctx, attemptID, userID)
	if err != nil {
		return nil, err
	}

	return dto.UnmarshalAttemptHistory(history.Data)
}

func (s *service) AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	attempts, err := s.repo.GetExpiredActiveAttempts(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	if len(attempts) == 0 {
		return 0, nil
	}

	workers := maxAutoSubmitWorkers
	if len(attempts) < workers {
		workers = len(attempts)
	}

	jobs := make(chan *model.Attempt)
	errs := make(chan error, len(attempts))
	var submitted atomic.Int64

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for attempt := range jobs {
				if _, err := s.submitAttempt(ctx, attempt.UserID, attempt.ID, nil); err != nil {
					if errors.Is(err, ErrAttemptAlreadySubmitted) || errors.Is(err, ErrAttemptCancelled) {
						continue
					}
					errs <- err
					continue
				}
				submitted.Add(1)
			}
		}()
	}

	for _, attempt := range attempts {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			close(errs)
			return int(submitted.Load()), errors.Join(ctx.Err(), errors.Join(readErrors(errs)...))
		case jobs <- attempt:
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)

	return int(submitted.Load()), errors.Join(readErrors(errs)...)
}

func readErrors(errs <-chan error) []error {
	result := make([]error, 0, len(errs))
	for err := range errs {
		result = append(result, err)
	}
	return result
}

func (s *service) submitAttempt(ctx context.Context, userID string, attemptID uint, submittedAnswers *[]dto.SubmitAttemptAnswer) (*dto.AttemptResponse, error) {
	var response *dto.AttemptResponse
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		attempt, err := repo.GetAttemptForLifecycle(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		switch attempt.Status {
		case model.AttemptStatusSubmitted:
			if attempt.History == nil {
				return ErrAttemptHistoryNotFound
			}
			response, err = dto.UnmarshalAttemptHistory(attempt.History.Data)
			return err
		case model.AttemptStatusCancelled:
			return ErrAttemptCancelled
		case model.AttemptStatusInProgress:
		default:
			return ErrAttemptNotActive
		}

		questions := make(map[uint]*model.Question, attempt.TotalQuestions)
		for sectionIdx := range attempt.Exam.Sections {
			section := &attempt.Exam.Sections[sectionIdx]
			if attempt.SectionCode != nil && section.Code != *attempt.SectionCode {
				continue
			}
			for questionIdx := range section.Questions {
				question := &section.Questions[questionIdx]
				if len(attempt.PartCodes) > 0 && (question.Part == nil || !partInSlice(*question.Part, attempt.PartCodes)) {
					continue
				}
				questions[question.ID] = question
			}
		}

		now := time.Now()
		answers := make([]model.AttemptAnswer, 0)
		if submittedAnswers != nil {
			answers = make([]model.AttemptAnswer, 0, len(*submittedAnswers))
			for _, item := range *submittedAnswers {
				question := questions[item.QuestionID]
				if question == nil {
					return ErrQuestionNotBelongToAttempt
				}
				if item.SelectedOptionID == nil {
					continue
				}

				var selectedOption *model.Option
				for optionIdx := range question.Options {
					option := &question.Options[optionIdx]
					if option.ID == *item.SelectedOptionID {
						selectedOption = option
						break
					}
				}
				if selectedOption == nil {
					return ErrOptionNotBelongToQuestion
				}

				isCorrect := selectedOption.IsCorrect
				answeredAt := now
				answers = append(answers, model.AttemptAnswer{
					AttemptID:        attempt.ID,
					QuestionID:       question.ID,
					SelectedOptionID: item.SelectedOptionID,
					IsCorrect:        &isCorrect,
					AnsweredAt:       &answeredAt,
				})
			}
		} else {
			answers = make([]model.AttemptAnswer, 0, len(attempt.Answers))
			for _, savedAnswer := range attempt.Answers {
				if savedAnswer.SelectedOptionID == nil {
					continue
				}
				question := questions[savedAnswer.QuestionID]
				if question == nil {
					continue
				}

				var selectedOption *model.Option
				for optionIdx := range question.Options {
					option := &question.Options[optionIdx]
					if option.ID == *savedAnswer.SelectedOptionID {
						selectedOption = option
						break
					}
				}
				if selectedOption == nil {
					continue
				}

				isCorrect := selectedOption.IsCorrect
				answeredAt := savedAnswer.AnsweredAt
				if answeredAt == nil {
					value := now
					answeredAt = &value
				}
				answers = append(answers, model.AttemptAnswer{
					AttemptID:        attempt.ID,
					QuestionID:       question.ID,
					SelectedOptionID: savedAnswer.SelectedOptionID,
					IsCorrect:        &isCorrect,
					AnsweredAt:       answeredAt,
				})
			}
		}

		if err := repo.ReplaceAttemptAnswers(ctx, attempt.ID, answers); err != nil {
			return err
		}

		correctAnswers := 0
		for _, answer := range answers {
			if answer.IsCorrect != nil && *answer.IsCorrect {
				correctAnswers++
			}
		}

		score, err := dto.CalculateAttemptScore(attempt.Exam.Type, answers, questions, attempt.TotalQuestions)
		if err != nil {
			return err
		}
		score = valuex.RoundFloat(score, 2)

		attempt.Status = model.AttemptStatusSubmitted
		attempt.SubmittedAt = &now
		attempt.CorrectAnswers = &correctAnswers
		attempt.Score = &score
		attempt.Answers = answers
		if err := repo.UpdateAttempt(ctx, attempt); err != nil {
			return err
		}

		data, err := dto.MarshalAttemptHistory(attempt)
		if err != nil {
			return err
		}
		history := &model.History{
			Data:      datatypes.JSON(data),
			AttemptID: attempt.ID,
			ExamTitle: attempt.Exam.Title,
			ExamType:  attempt.Exam.Type,
			ExamYear:  attempt.Exam.Year,
		}
		if err := repo.UpsertHistory(ctx, history); err != nil {
			return err
		}
		attempt.History = history
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil && attempt.Score != nil {
			payload, err := json.Marshal(classroomAssignmentAttemptSubmittedPayload{
				StudentID:    userID,
				AssignmentID: *attempt.ContextID,
				AttemptID:    attempt.ID,
				Score:        *attempt.Score,
			})
			if err != nil {
				return err
			}

			if err := repo.CreateOutboxEvent(ctx, &model.OutboxEvent{
				Type:          model.OutboxEventTypeClassroomAssignmentAttemptSubmitted,
				AggregateID:   valuex.UintStringPtr(attempt.ID),
				Status:        model.OutboxEventStatusPending,
				Payload:       payload,
				NextAttemptAt: time.Now(),
			}); err != nil {
				return err
			}
		}
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *service) ProcessOutboxEvents(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	// Lấy một batch nhỏ trước, sau đó publish ngoài DB transaction.
	var processed atomic.Int64
	events, err := s.repo.ClaimPendingOutboxEvents(ctx, time.Now(), limit, outboxVisibilityTimeout)
	if err != nil {
		return 0, err
	}

	for _, event := range events {
		// Publish event nội bộ sang Classrooms.
		var publishErr error
		switch event.Type {
		case model.OutboxEventTypeClassroomAssignmentAttemptCreated:
			var payload classroomAssignmentAttemptCreatedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				publishErr = err
				break
			}
			_, publishErr = s.classroomsClient.CreateAssignmentSubmission(ctx, &classroomsClient.CreateAssignmentSubmissionRequest{
				StudentId:    payload.StudentID,
				AssignmentId: uint64(payload.AssignmentID),
				AttemptId:    uint64(payload.AttemptID),
			})
		case model.OutboxEventTypeClassroomAssignmentAttemptSubmitted:
			var payload classroomAssignmentAttemptSubmittedPayload
			if err := json.Unmarshal(event.Payload, &payload); err != nil {
				publishErr = err
				break
			}
			_, publishErr = s.classroomsClient.UpdateAssignmentSubmissionResult(ctx, &classroomsClient.UpdateAssignmentSubmissionResultRequest{
				StudentId:    payload.StudentID,
				AssignmentId: uint64(payload.AssignmentID),
				AttemptId:    uint64(payload.AttemptID),
				Score:        payload.Score,
			})
		default:
			publishErr = errors.New("[ERROR] Unknown outbox event type")
		}

		// Nếu publish lỗi, giữ event để retry với backoff. Quá số lần tối đa thì dừng retry.
		if publishErr != nil {
			attempts := event.Attempts + 1
			permanent := attempts >= maxOutboxAttempts
			retryDelay := 5 * time.Second
			if permanent {
				retryDelay = 0
			} else if attempts >= 6 {
				retryDelay = 5 * time.Minute
			} else if attempts > 1 {
				retryDelay = time.Duration(attempts*attempts) * 5 * time.Second
			}

			nextAttemptAt := time.Now().Add(retryDelay)
			if markErr := s.repo.MarkOutboxEventFailed(ctx, event, attempts, nextAttemptAt, publishErr.Error(), permanent); markErr != nil {
				logger.Log.Error().
					Err(markErr).
					Uint("event_id", event.ID).
					Str("event_type", string(event.Type)).
					Int("attempts", attempts).
					Str("publish_error", publishErr.Error()).
					Msg("[OUTBOX] Failed to mark event for retry")
				return int(processed.Load()), errors.Join(publishErr, markErr)
			}
			logEvent := logger.Log.Error().
				Err(publishErr).
				Uint("event_id", event.ID).
				Str("event_type", string(event.Type)).
				Int("attempts", attempts)
			if permanent {
				logEvent.Msg("[OUTBOX] Event reached max retry attempts")
			} else {
				logEvent.Time("next_attempt_at", nextAttemptAt).Msg("[OUTBOX] Failed to publish event")
			}
			continue
		}

		// Nếu publish thành công, đánh dấu processed để không claim lại.
		if err := s.repo.MarkOutboxEventProcessed(ctx, event, time.Now()); err != nil {
			logger.Log.Error().
				Err(err).
				Uint("event_id", event.ID).
				Str("event_type", string(event.Type)).
				Msg("[OUTBOX] Failed to mark event processed")
			return int(processed.Load()), err
		}
		logger.Log.Info().
			Uint("event_id", event.ID).
			Str("event_type", string(event.Type)).
			Msg("[OUTBOX] Event published")
		processed.Add(1)
	}

	return int(processed.Load()), nil
}

func (s *service) GetPendingOutboxEvents(ctx context.Context, query dto.GetPendingOutboxEventsQuery) ([]*dto.OutboxEventResponse, int, error) {
	events, total, err := s.repo.GetPendingOutboxEvents(ctx, query)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]*dto.OutboxEventResponse, len(events))
	for i, event := range events {
		responses[i] = dto.ToOutboxEventResponse(event)
	}

	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) ReconcileClassroomAssignmentSubmissions(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	attempts, err := s.repo.GetSubmittedClassroomAssignmentAttemptsForReconcile(ctx, limit)
	if err != nil {
		return 0, err
	}

	pendingEvents := make([]*model.OutboxEvent, 0, len(attempts))
	for _, attempt := range attempts {
		// Bỏ qua các attempt không có contextID hoặc score để tránh tạo event lỗi.
		if attempt.ContextID == nil || attempt.Score == nil {
			continue
		}

		syncStatus, err := s.classroomsClient.GetAssignmentSubmissionSyncStatus(ctx, &classroomsClient.GetAssignmentSubmissionSyncStatusRequest{
			StudentId:    attempt.UserID,
			AssignmentId: uint64(*attempt.ContextID),
			AttemptId:    uint64(attempt.ID),
			Score:        *attempt.Score,
		})
		if err != nil {
			return len(pendingEvents), err
		}
		if syncStatus.Synced {
			continue
		}

		payload, err := json.Marshal(classroomAssignmentAttemptSubmittedPayload{
			StudentID:    attempt.UserID,
			AssignmentID: *attempt.ContextID,
			AttemptID:    attempt.ID,
			Score:        *attempt.Score,
		})
		if err != nil {
			return len(pendingEvents), err
		}

		pendingEvents = append(pendingEvents, &model.OutboxEvent{
			Type:          model.OutboxEventTypeClassroomAssignmentAttemptSubmitted,
			AggregateID:   valuex.UintStringPtr(attempt.ID),
			Status:        model.OutboxEventStatusPending,
			Payload:       payload,
			NextAttemptAt: time.Now(),
		})
	}

	if len(pendingEvents) == 0 {
		return 0, nil
	}

	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		for _, event := range pendingEvents {
			if err := repo.CreateOutboxEvent(ctx, event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return len(pendingEvents), nil
}
func partInSet(part model.PartCode, parts map[model.PartCode]struct{}) bool {
	_, ok := parts[part]
	return ok
}

func partInSlice(part model.PartCode, parts []model.PartCode) bool {
	for _, allowedPart := range parts {
		if allowedPart == part {
			return true
		}
	}
	return false
}
