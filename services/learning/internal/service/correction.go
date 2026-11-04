package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

// correctPrivateExam là nhánh CorrectionOnly của updatePrivateExam (đề đã có bài
// làm): chỉ cho đổi đáp án đúng / loại câu khỏi điểm, chấm lại mọi bài đã nộp,
// gửi thông báo đổi điểm (source EXAM), cập nhật điểm submission và ghi audit — tất
// cả trong tx của repo. GRADEBOOK_UPDATED gom vào gradebookEvents, gửi sau commit.
func (s *service) correctPrivateExam(ctx context.Context, repo repository.Repository, id uint, exam *model.Exam, title string, req dto.UpdatePrivateExamRequest, before []byte, gradebookEvents *[]*gradebookEvent) (updated *model.Exam, err error) {
	if strings.TrimSpace(req.ActorID) == "" || strings.TrimSpace(req.Reason) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Correction actor and reason are required", nil)
	}
	if title != exam.Title || (exam.Description == nil) != (req.Description == nil) || (exam.Description != nil && req.Description != nil && *exam.Description != *req.Description) || len(req.Questions) != len(exam.Sections[0].Questions) {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] An exam with attempts only allows correcting the answer or changing score exclusion", nil)
	}
	questionsByID := make(map[uint]*model.Question, len(exam.Sections[0].Questions))
	for i := range exam.Sections[0].Questions {
		question := &exam.Sections[0].Questions[i]
		questionsByID[question.ID] = question
	}
	for _, input := range req.Questions {
		question := questionsByID[input.ID]
		if question == nil || strings.TrimSpace(input.Content) != question.Content || input.Explanation == nil || strings.TrimSpace(*input.Explanation) != question.Explanation || len(input.Options) != len(question.Options) || len(input.OptionIDs) != len(question.Options) {
			return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Question structure cannot be changed after an attempt exists", nil)
		}
		correctOptionID := uint(0)
		for index, option := range question.Options {
			if input.OptionIDs[index] != option.ID || strings.TrimSpace(input.Options[index]) != strings.TrimSpace(*option.Content) {
				return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Question options cannot be changed after an attempt exists", nil)
			}
			if index == input.CorrectOptionIndex {
				correctOptionID = option.ID
			}
		}
		if correctOptionID == 0 {
			return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Correct option is invalid", nil)
		}
		if err := repo.UpdateQuestionScoring(ctx, question.ID, input.ExcludedFromScore, correctOptionID); err != nil {
			return nil, err
		}
	}
	attempts, err := repo.GetSubmittedAttemptsByExamID(ctx, id)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		score, err := dto.CalculateAttemptScore(attempt.Exam.Type, attempt.Answers, questionMap, attempt.TotalQuestions)
		if err != nil {
			return nil, err
		}
		score = valuex.RoundFloat(score, 2)
		applyScore(attempt, correctAnswers, score)
		afterScore := *attempt.Score
		scoreChanges = append(scoreChanges, model.ExamAuditScoreChange{AttemptID: attempt.ID, StudentID: attempt.UserID, BeforeScore: beforeScore, AfterScore: &afterScore})
		if err := repo.UpdateAttempt(ctx, attempt); err != nil {
			return nil, err
		}
		if err := createNotificationOutbox(ctx, repo, model.NotificationSourceExam,
			fmt.Sprintf("exam:%d:regrade:attempt:%d:%d", id, attempt.ID, regradedAt.UnixNano()),
			[]string{attempt.UserID}, "Score updated", "Your exam score has been recalculated", "ATTEMPT", attempt.ID, 0, time.Time{}); err != nil {
			return nil, err
		}
		data, err := dto.MarshalAttemptHistory(attempt)
		if err != nil {
			return nil, err
		}
		if err := repo.UpsertHistory(ctx, &model.History{Data: datatypes.JSON(data), AttemptID: attempt.ID, ExamTitle: attempt.Exam.Title, ExamType: attempt.Exam.Type, ExamYear: attempt.Exam.Year}); err != nil {
			return nil, err
		}
		if attempt.ContextType == model.AttemptContextTypeClassroomAssignment && attempt.ContextID != nil {
			gradebook, err := updateAssignmentSubmissionResult(ctx, repo, attempt)
			if err != nil {
				return nil, err
			}
			*gradebookEvents = append(*gradebookEvents, gradebook)
		}
	}
	updated, err = repo.GetPrivateAssignmentExam(ctx, id)
	if err != nil {
		return nil, err
	}
	after, err := json.Marshal(updated)
	if err != nil {
		return nil, err
	}
	changes, err := json.Marshal(scoreChanges)
	if err != nil {
		return nil, err
	}
	return updated, repo.CreateExamAudit(ctx, &model.ExamAudit{ExamID: id, ActorID: req.ActorID, Reason: strings.TrimSpace(req.Reason), Before: datatypes.JSON(before), After: datatypes.JSON(after), ScoreChanges: datatypes.JSON(changes), RegradedAt: &regradedAt, Changes: auditChanges(req.Changes)})
}

// getExamAudits trả lịch sử sửa/đính chính của đề private (mới nhất trước).
func (s *service) getExamAudits(ctx context.Context, examID uint) ([]*model.ExamAudit, error) {
	return s.repo.GetExamAudits(ctx, examID)
}
