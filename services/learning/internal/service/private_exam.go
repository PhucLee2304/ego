package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

// Đề private chỉ phục vụ assignment: các method dưới đây là nội bộ (không có
// route HTTP), được phía assignment gọi trực tiếp, trong transaction của nó
// qua withRepo khi cần. Lỗi validate là *httpx.HTTPError 400 (httpx.FromError giữ nguyên).

func (s *service) createPrivateExam(ctx context.Context, req dto.CreatePrivateExamRequest) (*model.Exam, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Exam title is required", nil)
	}
	if len(req.Questions) == 0 {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Exam must contain at least one question", nil)
	}

	questions := make([]model.Question, len(req.Questions))
	optionKeys := []model.OptionKey{model.OptionA, model.OptionB, model.OptionC, model.OptionD}
	for i, input := range req.Questions {
		if strings.TrimSpace(input.Content) == "" || len(input.Options) != len(optionKeys) || input.CorrectOptionIndex < 0 || input.CorrectOptionIndex >= len(optionKeys) {
			return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Each question must have content, exactly four options, and one valid correct option", nil)
		}
		options := make([]model.Option, len(optionKeys))
		for optionIndex, key := range optionKeys {
			content := strings.TrimSpace(input.Options[optionIndex])
			if content == "" {
				return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Question options cannot be blank", nil)
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
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		return repo.CreateExam(ctx, exam)
	}); err != nil {
		return nil, err
	}
	return exam, nil
}

func (s *service) getPrivateExam(ctx context.Context, id uint) (*model.Exam, error) {
	return s.repo.GetPrivateAssignmentExam(ctx, id)
}

func (s *service) updatePrivateExam(ctx context.Context, id uint, req dto.UpdatePrivateExamRequest) (*model.Exam, error) {
	var updated *model.Exam
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
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
			return httpx.NewError(http.StatusBadRequest, "[ERROR] Exam title and at least one question are required", nil)
		}
		if req.CorrectionOnly {
			updated, err = s.correctPrivateExam(ctx, repo, id, exam, title, req, before)
			return err
		}
		optionKeys := []model.OptionKey{model.OptionA, model.OptionB, model.OptionC, model.OptionD}
		questions := make([]model.Question, len(req.Questions))
		for i, input := range req.Questions {
			if strings.TrimSpace(input.Content) == "" || len(input.Options) != 4 || input.CorrectOptionIndex < 0 || input.CorrectOptionIndex > 3 {
				return httpx.NewError(http.StatusBadRequest, "[ERROR] Each question must have content, exactly four options, and one valid correct option", nil)
			}
			options := make([]model.Option, 4)
			for j, value := range input.Options {
				content := strings.TrimSpace(value)
				if content == "" {
					return httpx.NewError(http.StatusBadRequest, "[ERROR] Question options cannot be blank", nil)
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
