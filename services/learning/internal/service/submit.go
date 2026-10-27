package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"ego/platform/httpx"
	"ego/platform/valuex"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"

	"gorm.io/datatypes"
)

const maxAutoSubmitWorkers = 10

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

func (s *service) AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	attempts, err := s.repo.GetExpiredActiveAttempts(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	finished, err := s.finishedListeningOnlyAttempts(ctx, limit)
	if err != nil {
		return 0, err
	}
	attempts = mergeAttempts(attempts, finished)
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

// finishedListeningOnlyAttempts: attempt chỉ có phần nghe đã quá hạn trang nghe cuối
// (người dùng đóng tab nên không còn request nào kích hoạt lazy advance).
func (s *service) finishedListeningOnlyAttempts(ctx context.Context, limit int) ([]*model.Attempt, error) {
	candidates, err := s.repo.GetActiveListeningOnlyAttempts(ctx, limit)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	finished := make([]*model.Attempt, 0, len(candidates))
	for _, attempt := range candidates {
		advanceListening(attempt, now, s.listeningGrace())
		if listeningOnlyFinished(attempt) {
			finished = append(finished, attempt)
		}
	}
	return finished, nil
}

func mergeAttempts(first, second []*model.Attempt) []*model.Attempt {
	seen := make(map[uint]bool, len(first))
	for _, attempt := range first {
		seen[attempt.ID] = true
	}
	for _, attempt := range second {
		if !seen[attempt.ID] {
			first = append(first, attempt)
		}
	}
	return first
}

func readErrors(errs <-chan error) []error {
	result := make([]error, 0, len(errs))
	for err := range errs {
		result = append(result, err)
	}
	return result
}

// submitAttempt chấm điểm và chốt một attempt: nộp tay (submittedAnswers khác nil,
// thay toàn bộ đáp án đã lưu) hoặc auto-submit/lazy auto-submit (nil, chấm lại
// đáp án đã lưu qua WS).
func (s *service) submitAttempt(ctx context.Context, userID string, attemptID uint, submittedAnswers *[]dto.SubmitAttemptAnswer) (*dto.AttemptResponse, error) {
	if submittedAnswers == nil && s.attemptPublisher != nil {
		s.attemptPublisher(userID, map[string]any{
			"type":    "ATTEMPT_SUBMITTING",
			"payload": map[string]any{"attemptId": attemptID},
		})
	}
	var response *dto.AttemptResponse
	var gradebook *gradebookEvent
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
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
		if submittedAnswers != nil && isStrict(attempt) {
			advanceListening(attempt, s.clock(), s.listeningGrace())
			filtered := strictSubmittedAnswers(attempt, *submittedAnswers)
			submittedAnswers = &filtered
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
			// Điểm ghi vào submission cùng tx (thay outbox CLASSROOM_ASSIGNMENT_ATTEMPT_SUBMITTED).
			gradebook, err = updateAssignmentSubmissionResult(ctx, repo, attempt)
			if err != nil {
				return err
			}
			if err := createNotificationOutbox(ctx, repo, model.NotificationSourceAttempt, fmt.Sprintf("attempt:%d:submitted", attempt.ID), []string{userID},
				"Assignment graded", "Your assignment was submitted and graded", "ATTEMPT", attempt.ID, 0, time.Time{}); err != nil {
				return err
			}
		}
		response = dto.ToAttemptHistoryResponse(attempt)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if gradebook != nil {
		s.publishGradebookEvents([]*gradebookEvent{gradebook})
	}
	if s.attemptPublisher != nil && response != nil {
		s.attemptPublisher(userID, map[string]any{
			"type":    "ATTEMPT_SUBMITTED",
			"payload": map[string]any{"attemptId": attemptID, "score": response.Score},
		})
	}
	return response, nil
}

// strictSubmittedAnswers chặn lách khóa qua body submit ở flow strict: câu đang khóa giữ
// đáp án đã lưu (bỏ qua body), câu đang mở lấy theo body; câu mở mà body không nhắc tới
// giữ đáp án đã lưu (client chỉ biết đáp án của các trang đã tải). Câu chưa tới trang
// không có đáp án đã lưu nên bị bỏ qua và tính sai. Câu không thuộc attempt giữ lại để
// kiểm tra phía sau trả ErrQuestionNotBelongToAttempt như cũ.
func strictSubmittedAnswers(attempt *model.Attempt, submitted []dto.SubmitAttemptAnswer) []dto.SubmitAttemptAnswer {
	result := make([]dto.SubmitAttemptAnswer, 0, len(submitted)+len(attempt.Answers))
	inBody := make(map[uint]bool, len(submitted))
	for _, item := range submitted {
		if pageOfQuestion(attempt, item.QuestionID) < 0 || questionAccess(attempt, item.QuestionID) {
			result = append(result, item)
			inBody[item.QuestionID] = true
		}
	}
	for _, saved := range attempt.Answers {
		if saved.SelectedOptionID == nil || pageOfQuestion(attempt, saved.QuestionID) < 0 || inBody[saved.QuestionID] {
			continue
		}
		result = append(result, dto.SubmitAttemptAnswer{QuestionID: saved.QuestionID, SelectedOptionID: saved.SelectedOptionID})
	}
	return result
}
