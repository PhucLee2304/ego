package service

import (
	"context"
	"errors"
	"time"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

// errFlowAttemptExpired báo attempt đã hết giờ; tx đóng rồi mới nộp bài (submitAttempt mở tx riêng).
var errFlowAttemptExpired = errors.New("attempt expired")

// withFlowAttempt khóa attempt, chạy lazy advance (ghi lại con trỏ nếu đổi) rồi gọi fn trong cùng tx.
func (s *service) withFlowAttempt(ctx context.Context, userID string, attemptID uint, fn func(repo repository.Repository, attempt *model.Attempt, now time.Time) error) *httpx.HTTPError {
	finished := false
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		attempt, err := repo.GetAttemptForFlowUpdate(ctx, attemptID, userID)
		if err != nil {
			return err
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptNotActive
		}
		now := s.clock()
		if attempt.ExpiresAt != nil && !now.Before(*attempt.ExpiresAt) {
			return errFlowAttemptExpired
		}
		if advanceListening(attempt, now, s.listeningGrace()) {
			if err := repo.UpdateAttemptListening(ctx, attempt); err != nil {
				return err
			}
		}
		finished = listeningOnlyFinished(attempt)
		return fn(repo, attempt, now)
	})
	if submitErr := s.submitIfListeningFinished(ctx, userID, attemptID, finished); submitErr != nil {
		return httpx.FromError(submitErr, "[ERROR] Failed to submit finished attempt")
	}
	if errors.Is(err, errFlowAttemptExpired) {
		if _, submitErr := s.submitAttempt(ctx, userID, attemptID, nil); submitErr != nil &&
			!errors.Is(submitErr, ErrAttemptAlreadySubmitted) {
			return httpx.FromError(submitErr, "[ERROR] Failed to submit expired attempt")
		}
		return ErrAttemptExpired
	}
	if err != nil {
		var httpErr *httpx.HTTPError
		if errors.As(err, &httpErr) {
			return httpErr
		}
		return httpx.FromError(err, "[ERROR] Failed to load attempt")
	}
	return nil
}

func (s *service) GetAttemptNavigator(ctx context.Context, userID string, attemptID uint) (*dto.AttemptNavigatorResponse, *httpx.HTTPError) {
	var response *dto.AttemptNavigatorResponse
	httpErr := s.withFlowAttempt(ctx, userID, attemptID, func(repo repository.Repository, attempt *model.Attempt, now time.Time) error {
		numbers, err := s.layoutNumbers(ctx, repo, attempt.Layout)
		if err != nil {
			return err
		}
		answered := answeredQuestions(attempt)

		response = &dto.AttemptNavigatorResponse{
			Flow:           string(model.AttemptFlowFree),
			ServerTime:     now,
			ExpiresAt:      attempt.ExpiresAt,
			TotalQuestions: attempt.TotalQuestions,
			AnsweredCount:  len(answered),
			Pages:          make([]*dto.AttemptNavigatorPage, 0, len(attempt.Layout)),
		}
		if attempt.Flow != "" {
			response.Flow = string(attempt.Flow)
		}
		if attempt.Stage != nil {
			stage := string(*attempt.Stage)
			response.Stage = &stage
		}
		if inListening(attempt) {
			page := attempt.ListeningPage
			response.CurrentPage = &page
			response.PageStartedAt = attempt.ListeningPageStartedAt
		}

		for _, page := range attempt.Layout {
			locked := !pageAccess(attempt, page.Index)
			item := &dto.AttemptNavigatorPage{
				Index:     page.Index,
				Section:   string(page.Section),
				Part:      dto.PartString(page.Part),
				Questions: make([]*dto.AttemptNavigatorQuestion, 0, len(page.QuestionIDs)),
			}
			for _, id := range page.QuestionIDs {
				_, isAnswered := answered[id]
				item.Questions = append(item.Questions, &dto.AttemptNavigatorQuestion{
					ID: id, Number: numbers[id], Answered: isAnswered, Locked: locked,
				})
			}
			response.Pages = append(response.Pages, item)
		}
		return nil
	})
	if httpErr != nil {
		return nil, httpErr
	}
	return response, nil
}

func (s *service) GetAttemptPage(ctx context.Context, userID string, attemptID uint, index int) (*dto.AttemptPageResponse, *httpx.HTTPError) {
	var response *dto.AttemptPageResponse
	httpErr := s.withFlowAttempt(ctx, userID, attemptID, func(repo repository.Repository, attempt *model.Attempt, now time.Time) error {
		if index < 0 || index >= len(attempt.Layout) {
			return ErrAttemptPageNotFound
		}
		if !pageAccess(attempt, index) {
			return ErrAttemptPageLocked
		}
		page := attempt.Layout[index]

		questions, err := repo.GetPageQuestions(ctx, page.QuestionIDs)
		if err != nil {
			return err
		}
		numbers, err := s.layoutNumbers(ctx, repo, attempt.Layout)
		if err != nil {
			return err
		}
		var group *model.Group
		if page.GroupID != nil {
			if group, err = repo.GetGroupByID(ctx, *page.GroupID); err != nil {
				return err
			}
		}

		selected := make(map[uint]*uint, len(attempt.Answers))
		for _, answer := range attempt.Answers {
			selected[answer.QuestionID] = answer.SelectedOptionID
		}
		// Attempt ở đây luôn ACTIVE: Part 1/2 chỉ hiện key A/B/C như đề thật.
		hideContent := page.Part != nil && (*page.Part == model.Part1 || *page.Part == model.Part2)

		response = &dto.AttemptPageResponse{
			Index:     page.Index,
			Section:   string(page.Section),
			Part:      dto.PartString(page.Part),
			Group:     dto.ToAttemptPageGroup(group),
			Questions: dto.ToAttemptPageQuestions(questions, numbers, selected, hideContent),
		}
		if inListening(attempt) {
			startedAt := attempt.StartedAt
			if attempt.ListeningPageStartedAt != nil {
				startedAt = *attempt.ListeningPageStartedAt
			}
			response.Listening = &dto.AttemptPageListening{
				ElapsedMs:    max(0, now.Sub(startedAt).Milliseconds()),
				GapMs:        page.GapMs,
				MinAdvanceAt: startedAt.Add(time.Duration(page.AudioMs)*time.Millisecond - listeningAdvanceTolerance),
			}
			response.NextAudioURL, err = nextAudioURL(ctx, repo, attempt.Layout, index, page.GroupID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if httpErr != nil {
		return nil, httpErr
	}
	return response, nil
}

// listeningAdvanceTolerance: client được tiến trang sớm hơn lúc hết audio tối đa 2s (mục 1.3).
const listeningAdvanceTolerance = 2 * time.Second

// nextAudioURL trả audio của trang nghe kế tiếp có group khác, để client tải trước.
func nextAudioURL(ctx context.Context, repo repository.Repository, layout []model.AttemptPage, index int, currentGroup *uint) (*string, error) {
	for next := index + 1; next < len(layout); next++ {
		page := layout[next]
		if page.Section != model.SectionCodeListening || page.GroupID == nil {
			return nil, nil
		}
		if currentGroup != nil && *page.GroupID == *currentGroup {
			continue
		}
		group, err := repo.GetGroupByID(ctx, *page.GroupID)
		if err != nil {
			return nil, err
		}
		return group.AudioURL, nil
	}
	return nil, nil
}

func (s *service) layoutNumbers(ctx context.Context, repo repository.Repository, layout []model.AttemptPage) (map[uint]int, error) {
	ids := make([]uint, 0)
	for _, page := range layout {
		ids = append(ids, page.QuestionIDs...)
	}
	if len(ids) == 0 {
		return map[uint]int{}, nil
	}
	numbers, err := repo.GetQuestionNumbers(ctx, ids)
	if err != nil {
		return nil, err
	}
	return dto.QuestionNumbers(layout, numbers), nil
}

func answeredQuestions(attempt *model.Attempt) map[uint]struct{} {
	answered := make(map[uint]struct{}, len(attempt.Answers))
	for _, answer := range attempt.Answers {
		if answer.SelectedOptionID != nil {
			answered[answer.QuestionID] = struct{}{}
		}
	}
	return answered
}
