package service

import (
	"context"
	"errors"

	"ego/platform/logger"
	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

// UpdateAttemptAnswer phục vụ WS ATTEMPT_ANSWER_UPDATED: lỗi trả về luôn là
// *platformsocket.HandlerError mang mã lỗi WS, handler gửi thẳng cho client.
func (s *service) UpdateAttemptAnswer(ctx context.Context, userID, connectionID string, attemptID uint, questionID uint, optionID *uint) (*dto.UpdateAttemptAnswerResponse, error) {
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

		// Flow strict: chỉ chọn được đáp án ở trang đang mở, để không sửa đáp án của trang nghe
		// đã qua. Câu không thuộc attempt để kiểm tra bên dưới trả QUESTION_NOT_IN_ATTEMPT.
		advanced := false
		if isStrict(attempt) {
			advanced = advanceListening(attempt, now, s.listeningGrace())
			finished = listeningOnlyFinished(attempt)
		}
		// Sau kiểm tra hết giờ và lazy advance (tính trong bộ nhớ): kết nối không giữ phiên
		// vẫn nhận ATTEMPT_EXPIRED và vẫn kích hoạt nộp bài chỉ có phần nghe đã nghe hết,
		// nhưng không ghi gì vào DB.
		if err := s.requireSessionOwner(attempt, connectionID); err != nil {
			return err
		}
		if advanced {
			if err := repo.UpdateAttemptListening(ctx, attempt); err != nil {
				return err
			}
		}
		if isStrict(attempt) && pageOfQuestion(attempt, questionID) >= 0 && !questionAccess(attempt, questionID) {
			return platformsocket.NewHandlerError(socket.ErrorCodeQuestionLocked, "[SOCKET] Question is locked")
		}

		matchedQuestion := findAttemptQuestion(attempt, questionID)
		if matchedQuestion == nil {
			return platformsocket.NewHandlerError(socket.ErrorCodeQuestionNotInAttempt, ErrQuestionNotBelongToAttempt.Message)
		}
		var matchedOption *model.Option
		if optionID != nil {
			if matchedOption = findOption(matchedQuestion, *optionID); matchedOption == nil {
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
	// Không gửi nội dung lỗi DB cho client; ghi log để tra cứu.
	logger.Log.Error().Err(err).Msg("[ERROR] Attempt socket request failed")
	return platformsocket.NewHandlerError(socket.ErrorCodeAttemptAnswerRejected, "[SOCKET] Answer rejected")
}

// partInSlice cho biết part có nằm trong danh sách part đã chọn của lượt làm bài không.
func partInSlice(part model.PartCode, parts []model.PartCode) bool {
	for _, allowedPart := range parts {
		if allowedPart == part {
			return true
		}
	}
	return false
}
