package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"ego/services/learning/internal/model"
)

// maxAutoSubmitWorkers giới hạn số lượt nộp chạy song song mỗi vòng worker, để không chiếm hết
// kết nối DB khi nhiều bài hết giờ cùng lúc.
const maxAutoSubmitWorkers = 10

// AutoSubmitExpiredAttempts chạy định kỳ từ worker: nộp các lượt đã hết giờ, và các lượt TOEIC
// chỉ có phần nghe đã nghe xong (không còn gì để làm). Nộp song song nhiều lượt, trả số lượt
// đã nộp; lỗi của từng lượt được gom lại, không chặn các lượt khác.
func (s *service) AutoSubmitExpiredAttempts(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}

	attempts, err := s.repo.GetExpiredActiveAttempts(ctx, s.clock(), limit)
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

// mergeAttempts gộp hai danh sách lượt làm bài, bỏ trùng theo ID, để một lượt vừa hết giờ vừa
// nghe xong không bị nộp hai lần trong cùng vòng.
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

// readErrors gom lỗi từ channel (đã đóng) thành slice.
func readErrors(errs <-chan error) []error {
	result := make([]error, 0, len(errs))
	for err := range errs {
		result = append(result, err)
	}
	return result
}
