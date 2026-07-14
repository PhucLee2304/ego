package service

import (
	"context"
	platformdb "ego/platform/db"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Service interface {
	GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, error)
	GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, error)
	GetQuestions(ctx context.Context, id uint, query dto.GetExamQuestionsQuery) (*dto.GetExamQuestionsResponse, error)
	CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.CreateExamAttemptResponse, error)
}

type service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetList(ctx context.Context, query dto.GetExamsQuery) ([]*dto.GetExamsResponse, int, error) {
	exams, total, err := s.repo.GetList(ctx, query.Type, query.Limit(), query.Offset())
	if err != nil {
		return nil, 0, err
	}

	examDTOs := make([]*dto.GetExamsResponse, len(exams))
	for i, exam := range exams {
		examDTOs[i] = dto.ToGetExamsResponse(exam)
	}

	pageCounts := int((total + int64(query.PageSize) - 1) / int64(query.PageSize))
	if pageCounts == 0 {
		pageCounts = 1
	}

	return examDTOs, pageCounts, nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*dto.GetExamResponse, error) {
	exam, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return dto.ToGetExamResponse(exam), nil
}

func (s *service) GetQuestions(ctx context.Context, id uint, query dto.GetExamQuestionsQuery) (*dto.GetExamQuestionsResponse, error) {
	exam, err := s.repo.GetQuestionsByExamID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := dto.ValidateGetExamQuestionsQueryForExam(exam.Type, query); err != nil {
		return nil, err
	}

	return dto.ToGetExamQuestionsResponse(exam, query), nil
}

func (s *service) CreateAttempt(ctx context.Context, userID string, examID uint, req dto.CreateExamAttemptRequest) (*dto.CreateExamAttemptResponse, error) {
	var response *dto.CreateExamAttemptResponse
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if _, err := repo.GetActiveAttemptByUserID(ctx, userID); err == nil {
			return errors.New("[CONFLICT] Active attempt already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		exam, err := repo.GetAttemptExamByID(ctx, examID)
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
			value := int(*req.Duration)
			duration = &value
		}

		var expiresAt *time.Time
		if duration != nil {
			value := now.Add(time.Duration(*duration) * time.Minute)
			expiresAt = &value
		}

		var sectionCode *model.SectionCode
		var partCode *model.PartCode
		if req.Part != nil {
			value, ok := model.SectionCodeByPart(*req.Part)
			if !ok {
				return errors.New("[ERROR] Invalid part")
			}

			sectionCode = &value
			partValue := *req.Part
			partCode = &partValue
		} else if req.Section != nil {
			value := *req.Section
			sectionCode = &value
		} else if exam.Type == model.ExamTypeTHPT && req.Mode == model.AttemptModePractice {
			value := model.SectionCodeFull
			sectionCode = &value
		}

		totalQuestions := 0
		for _, section := range exam.Sections {
			if sectionCode != nil && section.Code != *sectionCode {
				continue
			}

			for _, question := range section.Questions {
				if partCode != nil && (question.Part == nil || *question.Part != *partCode) {
					continue
				}
				totalQuestions++
			}
		}

		attempt := &model.Attempt{
			Mode:           req.Mode,
			Status:         model.AttemptStatusInProgress,
			SectionCode:    sectionCode,
			PartCode:       partCode,
			StartedAt:      now,
			ExpiresAt:      expiresAt,
			SubmittedAt:    nil,
			Duration:       duration,
			TotalQuestions: totalQuestions,
			CorrectAnswers: nil,
			Score:          nil,
			ExamID:         exam.ID,
			UserID:         userID,
		}

		if err := repo.CreateAttempt(ctx, attempt); err != nil {
			return err
		}

		response = dto.ToCreateExamAttemptResponse(attempt)
		return nil
	})
	if err != nil {
		if platformdb.IsUniqueViolation(err, model.AttemptActiveUserUniqueIndex) {
			return nil, errors.New("[CONFLICT] Active attempt already exists")
		}
		return nil, err
	}

	return response, nil
}
