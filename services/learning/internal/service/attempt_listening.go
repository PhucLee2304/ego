package service

import (
	"context"
	"time"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"
)

// AdvanceListening phục vụ WS ATTEMPT_LISTENING_ADVANCE: chạy lazy advance, rồi nhận yêu cầu
// tiến trang nếu page == ListeningPage và now ≥ startedAt + audioMs − 2s. Khi nhận,
// startedAt của trang mới = now để độ trễ tải audio không cộng dồn (mục 1.3).
func (s *service) AdvanceListening(ctx context.Context, userID string, attemptID uint, page int) (*dto.ListeningStateResponse, error) {
	var response *dto.ListeningStateResponse
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := repo.GetAttemptForFlowUpdate(ctx, attemptID, userID)
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

		// Từ chối thì tx rollback, kể cả lazy advance: lần đọc sau tính lại y hệt.
		advanceListening(attempt, now, s.listeningGrace())
		if accepted, rejectCode := acceptListeningAdvance(attempt, page, now); !accepted {
			message := "[SOCKET] Listening page does not match current page"
			if rejectCode == socket.ErrorCodeListeningAdvanceTooEarly {
				message = "[SOCKET] Listening advance is too early"
			}
			return platformsocket.NewHandlerError(rejectCode, message)
		}
		if err := repo.UpdateAttemptListening(ctx, attempt); err != nil {
			return err
		}
		response = toListeningState(attempt, now)
		return nil
	})
	if err != nil {
		return nil, toAnswerSocketError(err)
	}
	return response, nil
}

// acceptListeningAdvance áp yêu cầu tiến trang của client lên attempt (hàm thuần).
// Trả mã lỗi WS khi từ chối; khi nhận thì attempt đã được cập nhật.
func acceptListeningAdvance(attempt *model.Attempt, page int, now time.Time) (bool, platformsocket.ErrorCode) {
	if !inListening(attempt) || page != attempt.ListeningPage {
		return false, socket.ErrorCodeListeningPageMismatch
	}
	startedAt := attempt.StartedAt
	if attempt.ListeningPageStartedAt != nil {
		startedAt = *attempt.ListeningPageStartedAt
	}
	minAdvanceAt := startedAt.Add(time.Duration(attempt.Layout[page].AudioMs)*time.Millisecond - listeningAdvanceTolerance)
	if now.Before(minAdvanceAt) {
		return false, socket.ErrorCodeListeningAdvanceTooEarly
	}

	attempt.ListeningPage++
	attempt.ListeningPageStartedAt = &now
	if attempt.ListeningPage >= listeningPageCount(attempt.Layout) {
		stage := model.AttemptStageReading
		attempt.Stage = &stage
	}
	return true, ""
}

func toListeningState(attempt *model.Attempt, now time.Time) *dto.ListeningStateResponse {
	response := &dto.ListeningStateResponse{ServerTime: now}
	if attempt.Stage != nil {
		response.Stage = string(*attempt.Stage)
	}
	if inListening(attempt) {
		page := attempt.ListeningPage
		response.CurrentPage = &page
		response.PageStartedAt = attempt.ListeningPageStartedAt
	}
	return response
}
