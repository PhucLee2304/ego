package service

import (
	"context"
	"errors"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

// UpdateAttemptAnswer phục vụ WS ATTEMPT_ANSWER_UPDATED: lỗi trả về luôn là
// *platformsocket.HandlerError mang mã lỗi WS, handler gửi thẳng cho client.
func (s *service) UpdateAttemptAnswer(ctx context.Context, userID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error) {
	var response *dto.UpdateAttemptAnswerResponse
	finished := false
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := repo.GetAttemptForAnswerUpdate(ctx, attemptID, userID)
		if err != nil {
			return err
		}

		if attempt.Status != model.AttemptStatusInProgress {
			return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotActive, ErrAttemptNotActive.Message)
		}

		now := s.clock()
		if attempt.ExpiresAt != nil && !now.Before(*attempt.ExpiresAt) {
			return platformsocket.NewHandlerError(socket.ErrorCodeAttemptExpired, ErrAttemptExpired.Message)
		}

		// Flow strict: chỉ chọn được đáp án ở trang đang mở (mục 1.4). Câu không thuộc
		// attempt để kiểm tra bên dưới trả QUESTION_NOT_IN_ATTEMPT như cũ.
		if isStrict(attempt) {
			if advanceListening(attempt, now, s.listeningGrace()) {
				if err := repo.UpdateAttemptListening(ctx, attempt); err != nil {
					return err
				}
			}
			finished = listeningOnlyFinished(attempt)
			if pageOfQuestion(attempt, questionID) >= 0 && !questionAccess(attempt, questionID) {
				return platformsocket.NewHandlerError(socket.ErrorCodeQuestionLocked, "[SOCKET] Question is locked")
			}
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
			return platformsocket.NewHandlerError(socket.ErrorCodeQuestionNotInAttempt, ErrQuestionNotBelongToAttempt.Message)
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
				return platformsocket.NewHandlerError(socket.ErrorCodeOptionNotInQuestion, ErrOptionNotBelongToQuestion.Message)
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
	if submitErr := s.submitIfListeningFinished(ctx, userID, attemptID, finished); submitErr != nil {
		return nil, submitErr
	}
	if err != nil {
		return nil, toAnswerSocketError(err)
	}
	return response, nil
}

// toAnswerSocketError giữ nguyên lỗi WS đã có mã, đổi attempt không tồn tại
// thành ATTEMPT_NOT_FOUND, còn lại là ATTEMPT_ANSWER_REJECTED.
func toAnswerSocketError(err error) error {
	var handlerErr *platformsocket.HandlerError
	if errors.As(err, &handlerErr) {
		return handlerErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return platformsocket.NewHandlerError(socket.ErrorCodeAttemptNotFound, "[SOCKET] Attempt not found")
	}
	return platformsocket.NewHandlerError(socket.ErrorCodeAttemptAnswerRejected, err.Error())
}

func partInSlice(part model.PartCode, parts []model.PartCode) bool {
	for _, allowedPart := range parts {
		if allowedPart == part {
			return true
		}
	}
	return false
}
