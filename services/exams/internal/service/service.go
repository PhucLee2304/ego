package service

import (
	"context"
	classroomsClient "ego/api/gen/go/classrooms"
	notificationsClient "ego/api/gen/go/notifications"
	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/logger"
	"ego/platform/valuex"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Service interface {
	GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, *httpx.HTTPError)
	GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, *httpx.HTTPError)
	CreatePrivateExam(ctx context.Context, req dto.CreatePrivateExamRequest) (*model.Exam, error)
	DeletePrivateExamRollback(ctx context.Context, id uint) error
	CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int, *httpx.HTTPError)
	GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, *httpx.HTTPError)
	UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error)
	SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError)
	CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError)
	AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error)
	SubmitAssignmentAttempts(ctx context.Context, assignmentID uint) (int, error)
	GetAssignmentAttemptState(ctx context.Context, assignmentID uint) (int64, int64, error)
	UpdateAssignmentAttemptSchedule(ctx context.Context, assignmentID uint, duration int, dueAt time.Time) (int64, error)
	GetPrivateExam(ctx context.Context, id uint) (*model.Exam, error)
	UpdatePrivateExam(ctx context.Context, id uint, req dto.UpdatePrivateExamRequest) (*model.Exam, error)
	ResetAssignmentAttempt(ctx context.Context, assignmentID uint, studentID string) error
	ProcessOutboxEvents(ctx context.Context, limit int) (int, error)
	GetPendingOutboxEvents(ctx context.Context, query dto.GetPendingOutboxEventsQuery) ([]*dto.OutboxEventResponse, int, *httpx.HTTPError)
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)
	SetNotificationClient(notificationsClient.NotificationServiceClient)
	ReconcileClassroomAssignmentSubmissions(ctx context.Context, limit int) (int, error)
	ReconcileAssignmentSubmissions(ctx context.Context, assignmentID uint) (int, error)
	SubmitStudentAssignmentAttempts(ctx context.Context, studentID string, assignmentIDs []uint) (int, error)
	GetExamAudits(ctx context.Context, examID uint) ([]*model.ExamAudit, error)
	GetAssignmentQuestionAnalytics(ctx context.Context, assignmentID uint, examID uint) ([]*repository.AssignmentQuestionAnalytics, error)
	SetAttemptEventPublisher(publisher func(userID string, payload any))
}

func (s *service) GetAssignmentQuestionAnalytics(ctx context.Context, assignmentID uint, examID uint) ([]*repository.AssignmentQuestionAnalytics, error) {
	return s.repo.GetAssignmentQuestionAnalytics(ctx, assignmentID, examID)
}

type service struct {
	repo                *repository.Repository
	classroomsClient    classroomsClient.ClassroomServiceClient
	notificationsClient notificationsClient.NotificationServiceClient
	reconcileCursor     atomic.Uint64
	attemptPublisher    func(userID string, payload any)
}

func (s *service) SetNotificationClient(client notificationsClient.NotificationServiceClient) {
	s.notificationsClient = client
}

func (s *service) SetAttemptEventPublisher(publisher func(userID string, payload any)) {
	s.attemptPublisher = publisher
}

func New(repo *repository.Repository, classroomsClient classroomsClient.ClassroomServiceClient) Service {
	return &service{
		repo:             repo,
		classroomsClient: classroomsClient,
	}
}

var (
	ErrAttemptNotActive            = httpx.NewError(http.StatusBadRequest, "[ERROR] Attempt is not active", nil)
	ErrAttemptExpired              = httpx.NewError(http.StatusBadRequest, "[ERROR] Attempt has expired", nil)
	ErrAttemptAlreadySubmitted     = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt has already been submitted", nil)
	ErrAttemptCancelled            = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt has been cancelled", nil)
	ErrAssignmentAttemptCancel     = httpx.NewError(http.StatusBadRequest, "[ERROR] Classroom assignment attempts cannot be cancelled; submit the attempt instead", nil)
	ErrQuestionNotBelongToAttempt  = httpx.NewError(http.StatusBadRequest, "[ERROR] Question does not belong to attempt", nil)
	ErrOptionNotBelongToQuestion   = httpx.NewError(http.StatusBadRequest, "[ERROR] Option does not belong to question", nil)
	ErrAttemptHistoryNotFound      = httpx.NewError(http.StatusNotFound, "[ERROR] Attempt history not found", nil)
	ErrContextAttemptAlreadyExists = httpx.NewError(http.StatusConflict, "[CONFLICT] Attempt already exists for this context", nil)
	ErrActiveAttemptExists         = httpx.NewError(http.StatusConflict, "[CONFLICT] Active attempt already exists", nil)
	ErrUnsupportedExamType         = httpx.NewError(http.StatusBadRequest, "[ERROR] Unsupported exam type", nil)
)

// PrivateExamValidationError marks a private exam request the client must fix;
// its Message is safe to show to end users.
type PrivateExamValidationError struct {
	Message string
}

func (e *PrivateExamValidationError) Error() string { return e.Message }

func invalidPrivateExam(message string) error {
	return &PrivateExamValidationError{Message: message}
}

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

func (s *service) GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, *httpx.HTTPError) {
	exams, total, err := s.repo.GetListExam(ctx, query.Type, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get exams", err)
	}

	examDTOs := make([]*dto.GetExamsResponse, len(exams))
	for i, exam := range exams {
		examDTOs[i] = dto.ToGetExamsResponse(exam)
	}

	return examDTOs, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, *httpx.HTTPError) {
	exam, err := s.repo.GetExamByID(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get exam")
	}

	return dto.ToGetExamResponse(exam), nil
}

func (s *service) CreatePrivateExam(ctx context.Context, req dto.CreatePrivateExamRequest) (*model.Exam, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, invalidPrivateExam("[ERROR] Exam title is required")
	}
	if len(req.Questions) == 0 {
		return nil, invalidPrivateExam("[ERROR] Exam must contain at least one question")
	}

	questions := make([]model.Question, len(req.Questions))
	optionKeys := []model.OptionKey{model.OptionA, model.OptionB, model.OptionC, model.OptionD}
	for i, input := range req.Questions {
		if strings.TrimSpace(input.Content) == "" || len(input.Options) != len(optionKeys) || input.CorrectOptionIndex < 0 || input.CorrectOptionIndex >= len(optionKeys) {
			return nil, invalidPrivateExam("[ERROR] Each question must have content, exactly four options, and one valid correct option")
		}
		options := make([]model.Option, len(optionKeys))
		for optionIndex, key := range optionKeys {
			content := strings.TrimSpace(input.Options[optionIndex])
			if content == "" {
				return nil, invalidPrivateExam("[ERROR] Question options cannot be blank")
			}
			options[optionIndex] = model.Option{
				Key:       key,
				Content:   &content,
				IsCorrect: optionIndex == input.CorrectOptionIndex,
				Order:     optionIndex + 1,
			}
		}
		explanation := ""
		if input.Explanation != nil {
			explanation = strings.TrimSpace(*input.Explanation)
		}
		questions[i] = model.Question{
			Content:           strings.TrimSpace(input.Content),
			Explanation:       explanation,
			Order:             i + 1,
			ExcludedFromScore: false,
			Options:           options,
		}
	}

	exam := &model.Exam{
		Title:       req.Title,
		Description: req.Description,
		IsPublic:    false,
		Type:        model.ExamTypeAssignment,
		Sections: []model.Section{{
			Code:      model.SectionCodeFull,
			Title:     "Assignment",
			Order:     1,
			Questions: questions,
		}},
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		return repo.CreateExam(ctx, exam)
	}); err != nil {
		return nil, err
	}
	return exam, nil
}

func (s *service) DeletePrivateExamRollback(ctx context.Context, id uint) error {
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		return repo.DeletePrivateExam(ctx, id)
	})
}

func (s *service) GetPrivateExam(ctx context.Context, id uint) (*model.Exam, error) {
	return s.repo.GetPrivateAssignmentExam(ctx, id)
}

func (s *service) UpdatePrivateExam(ctx context.Context, id uint, req dto.UpdatePrivateExamRequest) (*model.Exam, error) {
	var updated *model.Exam
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		exam, err := repo.GetPrivateAssignmentExam(ctx, id)
		if err != nil {
			return err
		}
		before, err := json.Marshal(exam)
		if err != nil {
			return err
		}
		title := strings.TrimSpace(req.Title)
		if title == "" || len(req.Questions) == 0 {
			return invalidPrivateExam("[ERROR] Exam title and at least one question are required")
		}
		if req.CorrectionOnly {
			if strings.TrimSpace(req.ActorID) == "" || strings.TrimSpace(req.Reason) == "" {
				return invalidPrivateExam("[ERROR] Correction actor and reason are required")
			}
			if title != exam.Title || (exam.Description == nil) != (req.Description == nil) || (exam.Description != nil && req.Description != nil && *exam.Description != *req.Description) || len(req.Questions) != len(exam.Sections[0].Questions) {
				return invalidPrivateExam("[ERROR] An exam with attempts only allows correcting the answer or changing score exclusion")
			}
			questionsByID := make(map[uint]*model.Question, len(exam.Sections[0].Questions))
			for i := range exam.Sections[0].Questions {
				question := &exam.Sections[0].Questions[i]
				questionsByID[question.ID] = question
			}
			for _, input := range req.Questions {
				question := questionsByID[input.ID]
				if question == nil || strings.TrimSpace(input.Content) != question.Content || input.Explanation == nil || strings.TrimSpace(*input.Explanation) != question.Explanation || len(input.Options) != len(question.Options) || len(input.OptionIDs) != len(question.Options) {
					return invalidPrivateExam("[ERROR] Question structure cannot be changed after an attempt exists")
				}
				correctOptionID := uint(0)
				for index, option := range question.Options {
					if input.OptionIDs[index] != option.ID || strings.TrimSpace(input.Options[index]) != strings.TrimSpace(*option.Content) {
						return invalidPrivateExam("[ERROR] Question options cannot be changed after an attempt exists")
					}
					if index == input.CorrectOptionIndex {
						correctOptionID = option.ID
					}
				}
				if correctOptionID == 0 {
					return invalidPrivateExam("[ERROR] Correct option is invalid")
				}
				if err := repo.UpdateQuestionScoring(ctx, question.ID, input.ExcludedFromScore, correctOptionID); err != nil {
					return err
				}
			}
			attempts, err := repo.GetSubmittedAttemptsByExamID(ctx, id)
			if err != nil {
				return err
			}
			scoreChanges := make([]model.ExamAuditScoreChange, 0, len(attempts))
			regradedAt := time.Now()
			for _, attempt := range attempts {
				var beforeScore *float64
				if attempt.Score != nil {
					value := *attempt.Score
					beforeScore = &value
				}
				questionMap := make(map[uint]*model.Question)
				for i := range attempt.Exam.Sections[0].Questions {
					question := &attempt.Exam.Sections[0].Questions[i]
					questionMap[question.ID] = question
				}
				correctAnswers := 0
				for i := range attempt.Answers {
					answer := &attempt.Answers[i]
					question := questionMap[answer.QuestionID]
					isCorrect := false
					if question != nil && answer.SelectedOptionID != nil {
						for _, option := range question.Options {
							if option.ID == *answer.SelectedOptionID {
								isCorrect = option.IsCorrect
							}
						}
					}
					answer.IsCorrect = &isCorrect
					if question != nil && !question.ExcludedFromScore && isCorrect {
						correctAnswers++
					}
				}
				if err := repo.ReplaceAttemptAnswers(ctx, attempt.ID, attempt.Answers); err != nil {
					return err
				}
				score, err := dto.CalculateAttemptScore(attempt.Exam.Type, attempt.Answers, questionMap, attempt.TotalQuestions)
				if err != nil {
					return err
				}
				score = valuex.RoundFloat(score, 2)
				attempt.CorrectAnswers, attempt.Score = &correctAnswers, &score
				afterScore := score
				scoreChanges = append(scoreChanges, model.ExamAuditScoreChange{AttemptID: attempt.ID, StudentID: attempt.UserID, BeforeScore: beforeScore, AfterScore: &afterScore})
				if err := repo.UpdateAttempt(ctx, attempt); err != nil {
					return err
				}
				if err := createNotificationOutbox(ctx, repo,
					fmt.Sprintf("exam:%d:regrade:attempt:%d:%d", id, attempt.ID, regradedAt.UnixNano()),
					attempt.UserID, "Score updated", "Your exam score has been recalculated", "ATTEMPT", attempt.ID); err != nil {
					return err
				}
				data, err := dto.MarshalAttemptHistory(attempt)
				if err != nil {
					return err
				}
				if err := repo.UpsertHistory(ctx, &model.History{Data: datatypes.JSON(data), AttemptID: attempt.ID, ExamTitle: attempt.Exam.Title, ExamType: attempt.Exam.Type, ExamYear: attempt.Exam.Year}); err != nil {
					return err
				}
				if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
					payload, err := json.Marshal(classroomAssignmentAttemptSubmittedPayload{StudentID: attempt.UserID, AssignmentID: *attempt.ContextID, AttemptID: attempt.ID, Score: score})
					if err != nil {
						return err
					}
					if err := repo.RequeueOrCreateOutboxEvent(ctx, &model.OutboxEvent{Type: model.OutboxEventTypeClassroomAssignmentAttemptSubmitted, AggregateID: valuex.UintStringPtr(attempt.ID), Status: model.OutboxEventStatusPending, Payload: payload, NextAttemptAt: time.Now()}); err != nil {
						return err
					}
				}
			}
			updated, err = repo.GetPrivateAssignmentExam(ctx, id)
			if err != nil {
				return err
			}
			after, err := json.Marshal(updated)
			if err != nil {
				return err
			}
			changes, err := json.Marshal(scoreChanges)
			if err != nil {
				return err
			}
			return repo.CreateExamAudit(ctx, &model.ExamAudit{ExamID: id, ActorID: req.ActorID, Reason: strings.TrimSpace(req.Reason), Before: datatypes.JSON(before), After: datatypes.JSON(after), ScoreChanges: datatypes.JSON(changes), RegradedAt: &regradedAt})
		}
		optionKeys := []model.OptionKey{model.OptionA, model.OptionB, model.OptionC, model.OptionD}
		questions := make([]model.Question, len(req.Questions))
		for i, input := range req.Questions {
			if strings.TrimSpace(input.Content) == "" || len(input.Options) != 4 || input.CorrectOptionIndex < 0 || input.CorrectOptionIndex > 3 {
				return invalidPrivateExam("[ERROR] Each question must have content, exactly four options, and one valid correct option")
			}
			options := make([]model.Option, 4)
			for j, value := range input.Options {
				content := strings.TrimSpace(value)
				if content == "" {
					return invalidPrivateExam("[ERROR] Question options cannot be blank")
				}
				options[j] = model.Option{Key: optionKeys[j], Content: &content, IsCorrect: j == input.CorrectOptionIndex, Order: j + 1}
			}
			explanation := ""
			if input.Explanation != nil {
				explanation = strings.TrimSpace(*input.Explanation)
			}
			questions[i] = model.Question{Content: strings.TrimSpace(input.Content), Explanation: explanation, Order: i + 1, ExcludedFromScore: input.ExcludedFromScore, Options: options}
		}
		exam.Title, exam.Description = title, req.Description
		if err := repo.ReplacePrivateExamQuestions(ctx, exam, questions); err != nil {
			return err
		}
		updated, err = repo.GetPrivateAssignmentExam(ctx, id)
		if err != nil {
			return err
		}
		after, err := json.Marshal(updated)
		if err != nil {
			return err
		}
		return repo.CreateExamAudit(ctx, &model.ExamAudit{ExamID: id, ActorID: req.ActorID, Reason: strings.TrimSpace(req.Reason), Before: datatypes.JSON(before), After: datatypes.JSON(after), ScoreChanges: datatypes.JSON("[]")})
	})
	return updated, err
}

func (s *service) ResetAssignmentAttempt(ctx context.Context, assignmentID uint, studentID string) error {
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		return repo.ResetAssignmentAttempt(ctx, assignmentID, studentID)
	})
}

func (s *service) CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError) {
	var assignmentValidation *classroomsClient.ValidateAssignmentAttemptResponse
	if req.ContextType == model.AttemptContextTypeClassroomAssignment {
		resp, err := s.classroomsClient.ValidateAssignmentAttempt(ctx, &classroomsClient.ValidateAssignmentAttemptRequest{
			UserId:       userID,
			AssignmentId: uint64(*req.ContextID),
			ExamId:       uint64(examID),
		})
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
		}
		assignmentValidation = resp
	}

	activeAttempt, activeErr := s.repo.GetActiveAttemptByUserID(ctx, userID)
	// Submitting an expired attempt completes its own transaction before the
	// create transaction starts. The unique active-attempt index remains the
	// final guard when concurrent create requests race here.
	if activeErr == nil && activeAttempt.ExpiresAt != nil && !time.Now().Before(*activeAttempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, activeAttempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
		}
	} else if activeErr != nil && !errors.Is(activeErr, gorm.ErrRecordNotFound) {
		return nil, httpx.FromError(activeErr, "[ERROR] Failed to create attempt")
	}

	var attempt *model.Attempt
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if _, err := repo.GetActiveAttemptByUserID(ctx, userID); err == nil {
			return ErrActiveAttemptExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		exam, err := repo.GetQuestionsByExamID(ctx, examID)
		if err != nil {
			return err
		}

		if (exam.Type == model.ExamTypeTHPT || exam.Type == model.ExamTypeTOEIC) &&
			!exam.IsPublic &&
			req.ContextType != model.AttemptContextTypeClassroomAssignment {
			return gorm.ErrRecordNotFound
		}

		if err := dto.ValidateCreateExamAttemptRequestForExam(exam.Type, req); err != nil {
			return httpx.NewError(http.StatusBadRequest, err.Error(), err)
		}

		now := time.Now()
		var duration *int
		if req.Mode == model.AttemptModeAssignment {
			value := int(assignmentValidation.DurationMinutes)
			duration = &value
		} else if req.Mode == model.AttemptModeTest {
			if exam.Type != model.ExamTypeAssignment {
				value, ok := model.GetTestDurationByExamType(exam.Type)
				if !ok {
					return ErrUnsupportedExamType
				}
				duration = &value
			}
		} else if req.Duration != nil {
			duration = valuex.MapPtr(req.Duration, func(value model.AttemptDuration) int {
				return int(value)
			})
		}

		var expiresAt *time.Time
		if duration != nil {
			value := now.Add(time.Duration(*duration) * time.Minute)
			if assignmentValidation != nil {
				dueAt := time.Unix(assignmentValidation.DueAtUnix, 0)
				value = assignmentAttemptExpiresAt(now, *duration, dueAt)
			}
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
			for _, group := range section.Groups {
				for _, question := range group.Questions {
					if len(partSet) > 0 && (question.Part == nil || !partInSet(*question.Part, partSet)) {
						continue
					}
					totalQuestions++
				}
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
		attempt.Exam = *exam
		attempt.Answers = []model.AttemptAnswer{}

		return nil
	})
	if err != nil {
		if platformdb.IsUniqueViolation(err, model.AttemptActiveUserUniqueIndex) {
			return nil, ErrActiveAttemptExists
		}
		if platformdb.IsUniqueViolation(err, model.AttemptClassroomAssignmentUniqueIndex) {
			return nil, ErrContextAttemptAlreadyExists
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
	}
	if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
		if _, err := s.classroomsClient.CreateAssignmentSubmission(ctx, &classroomsClient.CreateAssignmentSubmissionRequest{
			StudentId: userID, AssignmentId: uint64(*attempt.ContextID), AttemptId: uint64(attempt.ID),
		}); err != nil {
			if cleanupErr := s.repo.DeleteAttemptRollback(ctx, attempt.ID); cleanupErr != nil {
				return nil, httpx.FromError(errors.Join(err, cleanupErr), "[ERROR] Failed to create attempt")
			}
			return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
		}
	}

	return dto.ToAttemptResponseWithQuestions(attempt), nil
}

func assignmentAttemptExpiresAt(startedAt time.Time, durationMinutes int, dueAt time.Time) time.Time {
	expiresAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
	if dueAt.Before(expiresAt) {
		return dueAt
	}
	return expiresAt
}

func (s *service) GetAttempts(ctx context.Context, userID string, query dto.GetAttemptsQuery) ([]*dto.AttemptResponse, int, *httpx.HTTPError) {
	attempts, total, err := s.repo.GetAttemptsByUserIDAndStatus(ctx, userID, query.Status, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get attempts", err)
	}

	now := time.Now()
	responses := make([]*dto.AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = dto.ToAttemptResponse(attempt, now)
	}

	return responses, httpx.ToPageCounts(total, query.PageSize), nil
}

func (s *service) GetAttemptQuestions(ctx context.Context, userID string, id uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	attempt, err := s.repo.GetAttemptQuestionsByIDAndUserID(ctx, id, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get attempt questions")
	}

	if attempt.Status != model.AttemptStatusInProgress {
		return nil, ErrAttemptNotActive
	}

	if attempt.ExpiresAt != nil && !time.Now().Before(*attempt.ExpiresAt) {
		if _, err := s.submitAttempt(ctx, userID, attempt.ID, nil); err != nil &&
			!errors.Is(err, ErrAttemptAlreadySubmitted) {
			return nil, httpx.FromError(err, "[ERROR] Failed to get attempt questions")
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
			if matchedQuestion == nil {
				for groupIdx := range section.Groups {
					group := &section.Groups[groupIdx]
					for questionIdx := range group.Questions {
						question := &group.Questions[questionIdx]
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

func (s *service) SubmitAttempt(ctx context.Context, userID string, attemptID uint, req dto.SubmitAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError) {
	response, err := s.submitAttempt(ctx, userID, attemptID, &req.Answers)
	if err != nil {
		if errors.Is(err, dto.ErrScoreQuestionNotFound) || errors.Is(err, dto.ErrScoreQuestionSectionEmpty) {
			return nil, httpx.NewError(http.StatusBadRequest, err.Error(), err)
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to submit attempt")
	}
	return response, nil
}

func (s *service) CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	attempt, err := s.repo.GetAttemptForLifecycle(ctx, attemptID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to cancel attempt")
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
		return nil, httpx.FromError(err, "[ERROR] Failed to cancel attempt")
	}

	return response, nil
}

func (s *service) GetAttemptHistory(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	history, err := s.repo.GetAttemptHistoryByIDAndUserID(ctx, attemptID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get attempt history")
	}

	response, err := dto.UnmarshalAttemptHistory(history.Data)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to read attempt history", err)
	}
	return response, nil
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

func (s *service) SubmitAssignmentAttempts(ctx context.Context, assignmentID uint) (int, error) {
	if err := s.repo.ExpireActiveAssignmentAttempts(ctx, assignmentID, time.Now()); err != nil {
		return 0, err
	}
	attempts, err := s.repo.GetActiveAttemptsByAssignmentID(ctx, assignmentID)
	if err != nil {
		return 0, err
	}

	submitted := 0
	var errs []error
	for _, attempt := range attempts {
		if _, err := s.submitAttempt(ctx, attempt.UserID, attempt.ID, nil); err != nil {
			if errors.Is(err, ErrAttemptAlreadySubmitted) || errors.Is(err, ErrAttemptCancelled) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		submitted++
	}
	if len(errs) > 0 {
		logger.Log.Error().Err(errors.Join(errs...)).Uint("assignment_id", assignmentID).
			Msg("[EXAMS] Some assignment attempts were queued for auto-submit retry")
	}
	return submitted, nil
}

func (s *service) SubmitStudentAssignmentAttempts(ctx context.Context, studentID string, assignmentIDs []uint) (int, error) {
	if len(assignmentIDs) == 0 {
		return 0, nil
	}
	if err := s.repo.ExpireActiveAssignmentAttemptsByUser(ctx, assignmentIDs, studentID, time.Now()); err != nil {
		return 0, err
	}
	attempts, err := s.repo.GetActiveAttemptsByAssignmentIDsAndUser(ctx, assignmentIDs, studentID)
	if err != nil {
		return 0, err
	}

	submitted := 0
	var errs []error
	for _, attempt := range attempts {
		if _, err := s.submitAttempt(ctx, studentID, attempt.ID, nil); err != nil {
			if errors.Is(err, ErrAttemptAlreadySubmitted) || errors.Is(err, ErrAttemptCancelled) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		submitted++
	}
	if len(errs) > 0 {
		return submitted, errors.Join(errs...)
	}
	return submitted, nil
}

func (s *service) GetExamAudits(ctx context.Context, examID uint) ([]*model.ExamAudit, error) {
	return s.repo.GetExamAudits(ctx, examID)
}

func (s *service) GetAssignmentAttemptState(ctx context.Context, assignmentID uint) (int64, int64, error) {
	return s.repo.CountAssignmentAttempts(ctx, assignmentID)
}

func (s *service) UpdateAssignmentAttemptSchedule(ctx context.Context, assignmentID uint, duration int, dueAt time.Time) (int64, error) {
	return s.repo.UpdateActiveAssignmentAttemptSchedule(ctx, assignmentID, duration, dueAt)
}

func readErrors(errs <-chan error) []error {
	result := make([]error, 0, len(errs))
	for err := range errs {
		result = append(result, err)
	}
	return result
}

func (s *service) submitAttempt(ctx context.Context, userID string, attemptID uint, submittedAnswers *[]dto.SubmitAttemptAnswer) (*dto.AttemptResponse, error) {
	if submittedAnswers == nil && s.attemptPublisher != nil {
		s.attemptPublisher(userID, map[string]any{
			"type":    "ATTEMPT_SUBMITTING",
			"payload": map[string]any{"attemptId": attemptID},
		})
	}
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
		if submittedAnswers != nil && attempt.ExpiresAt != nil && !time.Now().Before(*attempt.ExpiresAt) {
			return ErrAttemptExpired
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
			for groupIdx := range section.Groups {
				group := &section.Groups[groupIdx]
				for questionIdx := range group.Questions {
					question := &group.Questions[questionIdx]
					if len(attempt.PartCodes) > 0 && (question.Part == nil || !partInSlice(*question.Part, attempt.PartCodes)) {
						continue
					}
					questions[question.ID] = question
				}
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
			question := questions[answer.QuestionID]
			if question != nil && !question.ExcludedFromScore && answer.IsCorrect != nil && *answer.IsCorrect {
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
			if err := createNotificationOutbox(ctx, repo, fmt.Sprintf("attempt:%d:submitted", attempt.ID), userID,
				"Assignment graded", "Your assignment was submitted and graded", "ATTEMPT", attempt.ID); err != nil {
				return err
			}
		}
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.attemptPublisher != nil && response != nil {
		s.attemptPublisher(userID, map[string]any{
			"type":    "ATTEMPT_SUBMITTED",
			"payload": map[string]any{"attemptId": attemptID, "score": response.Score},
		})
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
				StudentId:     payload.StudentID,
				AssignmentId:  uint64(payload.AssignmentID),
				AttemptId:     uint64(payload.AttemptID),
				AllowInactive: true,
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

func (s *service) GetPendingOutboxEvents(ctx context.Context, query dto.GetPendingOutboxEventsQuery) ([]*dto.OutboxEventResponse, int, *httpx.HTTPError) {
	events, total, err := s.repo.GetPendingOutboxEvents(ctx, query)
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get pending outbox events", err)
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

	cursor := uint(s.reconcileCursor.Load())
	attempts, err := s.repo.GetSubmittedClassroomAssignmentAttemptsForReconcile(ctx, cursor, limit)
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
		if len(attempts) == 0 && cursor != 0 {
			s.reconcileCursor.Store(0)
		} else if len(attempts) > 0 {
			s.reconcileCursor.Store(uint64(attempts[len(attempts)-1].ID))
		}
		return 0, nil
	}

	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		for _, event := range pendingEvents {
			if err := repo.RequeueOrCreateOutboxEvent(ctx, event); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	s.reconcileCursor.Store(uint64(attempts[len(attempts)-1].ID))

	return len(pendingEvents), nil
}

func (s *service) ReconcileAssignmentSubmissions(ctx context.Context, assignmentID uint) (int, error) {
	attempts, err := s.repo.GetSubmittedAttemptsByAssignment(ctx, assignmentID)
	if err != nil {
		return 0, err
	}
	enqueued := 0
	for _, attempt := range attempts {
		if attempt.Score == nil {
			continue
		}
		status, err := s.classroomsClient.GetAssignmentSubmissionSyncStatus(ctx, &classroomsClient.GetAssignmentSubmissionSyncStatusRequest{StudentId: attempt.UserID, AssignmentId: uint64(assignmentID), AttemptId: uint64(attempt.ID), Score: *attempt.Score})
		if err != nil {
			return enqueued, err
		}
		if status.Synced {
			continue
		}
		payload, err := json.Marshal(classroomAssignmentAttemptSubmittedPayload{StudentID: attempt.UserID, AssignmentID: assignmentID, AttemptID: attempt.ID, Score: *attempt.Score})
		if err != nil {
			return enqueued, err
		}
		event := &model.OutboxEvent{Type: model.OutboxEventTypeClassroomAssignmentAttemptSubmitted, AggregateID: valuex.UintStringPtr(attempt.ID), Status: model.OutboxEventStatusPending, Payload: payload, NextAttemptAt: time.Now()}
		if err := s.repo.RequeueOrCreateOutboxEvent(ctx, event); err != nil {
			return enqueued, err
		}
		enqueued++
	}
	return enqueued, nil
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
