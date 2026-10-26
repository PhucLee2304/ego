package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	platformdb "ego/platform/db"
	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *service) CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.AttemptResponse, *httpx.HTTPError) {
	var assignment *model.Assignment
	if req.ContextType == model.AttemptContextTypeClassroomAssignment {
		var err error
		assignment, err = s.validateAssignmentAttempt(ctx, s.repo, userID, *req.ContextID, examID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to create attempt")
		}
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
	var gradebook *gradebookEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
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
			value := assignment.Duration
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
			if assignment != nil {
				value = assignmentAttemptExpiresAt(now, *duration, assignment.DueAt)
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
			Flow:           s.attemptFlowFor(exam, req.Mode),
			Layout:         buildAttemptLayout(exam, sectionCode, partCodes, s.listeningGap()),
		}
		if attempt.Flow == model.AttemptFlowTOEICStrict {
			stage := model.AttemptStageReading
			if hasListeningPage(attempt.Layout) {
				stage = model.AttemptStageListening
				attempt.ListeningPageStartedAt = &now
			}
			attempt.Stage = &stage
		}

		if err := repo.CreateAttempt(ctx, attempt); err != nil {
			return err
		}
		attempt.Exam = *exam
		attempt.Answers = []model.AttemptAnswer{}

		// Submission ghi cùng tx với attempt (thay RPC CreateAssignmentSubmission + DeleteAttemptRollback).
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
			gradebook, err = createAssignmentSubmission(ctx, repo, userID, *attempt.ContextID, attempt.ID)
			if err != nil {
				return err
			}
		}
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
	if gradebook != nil {
		s.publishGradebookEvents([]*gradebookEvent{gradebook})
	}
	// Flow strict lấy nội dung theo trang (T3), không trả cả cây đề.
	if attempt.Flow == model.AttemptFlowTOEICStrict {
		return dto.ToAttemptResponse(attempt, time.Now()), nil
	}
	return dto.ToAttemptResponseWithQuestions(attempt), nil
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
	if attempt.Flow == model.AttemptFlowTOEICStrict {
		return nil, ErrAttemptPagedFlow
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

func (s *service) CancelAttempt(ctx context.Context, userID string, attemptID uint) (*dto.AttemptResponse, *httpx.HTTPError) {
	attempt, err := s.repo.GetAttemptForLifecycle(ctx, attemptID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to cancel attempt")
	}
	if attempt.ContextType == model.AttemptContextTypeClassroomAssignment {
		return nil, ErrAssignmentAttemptCancel
	}

	var response *dto.AttemptResponse
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
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

func partInSet(part model.PartCode, parts map[model.PartCode]struct{}) bool {
	_, ok := parts[part]
	return ok
}
