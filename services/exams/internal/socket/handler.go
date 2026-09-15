package socket

import (
	"context"
	"errors"

	platformsocket "ego/platform/socket"
	"ego/services/exams/internal/service"

	"gorm.io/gorm"
)

type AttemptAnswerUpdatedPayload struct {
	AttemptID  uint  `json:"attemptId" validate:"required,gt=0"`
	QuestionID uint  `json:"questionId" validate:"required,gt=0"`
	OptionID   *uint `json:"optionId,omitempty"`
}

func handleAttemptAnswerUpdated(ctx context.Context, client *platformsocket.Client, svc service.Service, payload AttemptAnswerUpdatedPayload) (any, error) {
	resp, err := svc.UpdateAttemptAnswer(ctx, client.UserID(), payload.AttemptID, payload.QuestionID, payload.OptionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, platformsocket.NewHandlerError(ErrorCodeAttemptNotFound, "[SOCKET] Attempt not found")
		}

		var errCode platformsocket.ErrorCode
		switch {
		case errors.Is(err, service.ErrAttemptNotActive):
			errCode = ErrorCodeAttemptNotActive
		case errors.Is(err, service.ErrAttemptExpired):
			errCode = ErrorCodeAttemptExpired
		case errors.Is(err, service.ErrQuestionNotBelongToAttempt):
			errCode = ErrorCodeQuestionNotInAttempt
		case errors.Is(err, service.ErrOptionNotBelongToQuestion):
			errCode = ErrorCodeOptionNotInQuestion
		default:
			errCode = ErrorCodeAttemptAnswerRejected
		}

		return nil, platformsocket.NewHandlerError(errCode, err.Error())
	}

	return resp, nil
}
