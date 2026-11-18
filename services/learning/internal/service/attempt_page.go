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

// errFlowAttemptExpired là tín hiệu nội bộ từ trong transaction ra ngoài rằng lượt đã hết giờ.
// Không nộp bài ngay trong tx vì submitAttempt mở transaction riêng; tx đóng rồi mới nộp.
var errFlowAttemptExpired = errors.New("attempt expired")

// withFlowAttempt là khung chung của các API đọc bài theo trang (navigator, page): khóa attempt,
// đưa trạng thái về đúng thời điểm hiện tại, rồi gọi fn trong cùng transaction.
// "Lazy advance": server không có đồng hồ chạy nền cho từng bài, nên mỗi lần client hỏi thì
// tính lại trang nghe đáng lẽ đang ở theo giờ thật và ghi lại nếu đổi. Nhờ vậy reload hay mất
// mạng bao lâu cũng không nghe lại được trang đã qua.
// Sau transaction, lượt hết giờ hoặc chỉ có phần nghe đã nghe xong được nộp luôn.
func (s *service) withFlowAttempt(ctx context.Context, userID string, attemptID uint, fn func(repo repository.Repository, attempt *model.Attempt, now time.Time) error) *httpx.HTTPError {
	finished := false
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// Khóa dòng để lazy advance không chạy chen với tiến trang hay nộp bài ở request khác.
		attempt, err := repo.GetAttemptForFlowUpdate(ctx, attemptID, userID)
		if err != nil {
			return err
		}
		if attempt.Status != model.AttemptStatusInProgress {
			return ErrAttemptNotActive
		}
		// Hết giờ thì thoát tx bằng tín hiệu riêng để nộp bài sau khi tx đóng.
		now := s.clock()
		if attempt.ExpiresAt != nil && !now.Before(*attempt.ExpiresAt) {
			return errFlowAttemptExpired
		}
		// Tiến con trỏ nghe theo giờ thật, chỉ ghi DB khi thật sự đổi.
		if advanceListening(attempt, now, s.listeningGrace()) {
			if err := repo.UpdateAttemptListening(ctx, attempt); err != nil {
				return err
			}
		}
		// Ghi nhận lượt chỉ có phần nghe vừa nghe xong để nộp sau tx; fn vẫn chạy để trả trạng thái cuối cho client.
		finished = listeningOnlyFinished(attempt)
		return fn(repo, attempt, now)
	})
	// Không còn trang nào để làm thì nộp ngay, không chờ worker.
	if submitErr := s.submitIfListeningFinished(ctx, userID, attemptID, finished); submitErr != nil {
		return httpx.FromError(submitErr, "[ERROR] Failed to submit finished attempt")
	}
	// Hết giờ: nộp bằng đáp án đã lưu rồi báo hết giờ, để client chuyển sang màn kết quả. Bài đã
	// được nộp ở nơi khác thì bỏ qua.
	if errors.Is(err, errFlowAttemptExpired) {
		if _, submitErr := s.submitAttempt(ctx, userID, attemptID, nil); submitErr != nil &&
			!errors.Is(submitErr, ErrAttemptAlreadySubmitted) {
			return httpx.FromError(submitErr, "[ERROR] Failed to submit expired attempt")
		}
		return ErrAttemptExpired
	}
	// Lỗi nghiệp vụ fn trả (PAGE_LOCKED, không tìm thấy trang…) giữ nguyên status; lỗi khác là 500.
	if err != nil {
		var httpErr *httpx.HTTPError
		if errors.As(err, &httpErr) {
			return httpErr
		}
		return httpx.FromError(err, "[ERROR] Failed to load attempt")
	}
	return nil
}

// GetAttemptNavigator trả bảng chọn câu: danh sách trang, số câu, câu nào đã trả lời, trang nào
// đang khóa, cùng giờ server để client tính thời gian còn lại đúng dù đồng hồ máy lệch.
func (s *service) GetAttemptNavigator(ctx context.Context, userID string, attemptID uint) (*dto.AttemptNavigatorResponse, *httpx.HTTPError) {
	var response *dto.AttemptNavigatorResponse
	httpErr := s.withFlowAttempt(ctx, userID, attemptID, func(repo repository.Repository, attempt *model.Attempt, now time.Time) error {
		numbers, err := s.layoutNumbers(ctx, repo, attempt.Layout)
		if err != nil {
			return err
		}
		answered := answeredQuestions(attempt)

		// Thông tin chung: luồng, giờ server và hạn kết thúc để client đếm ngược đúng, tiến độ làm bài.
		response = &dto.AttemptNavigatorResponse{
			Flow:           string(model.AttemptFlowFree),
			ServerTime:     now,
			ExpiresAt:      attempt.ExpiresAt,
			TotalQuestions: attempt.TotalQuestions,
			AnsweredCount:  len(answered),
			Pages:          make([]*dto.AttemptNavigatorPage, 0, len(attempt.Layout)),
		}
		// Attempt cũ tạo trước khi có cột Flow để trống: coi như FREE.
		if attempt.Flow != "" {
			response.Flow = string(attempt.Flow)
		}
		if attempt.Stage != nil {
			stage := string(*attempt.Stage)
			response.Stage = &stage
		}
		// Đang ở phần nghe: báo trang nghe hiện tại và lúc nó bắt đầu, để client mở đúng trang và
		// đồng bộ đồng hồ của trang.
		if inListening(attempt) {
			page := attempt.ListeningPage
			response.CurrentPage = &page
			response.PageStartedAt = attempt.ListeningPageStartedAt
		}

		// Mỗi trang kèm danh sách câu với số thứ tự, đã trả lời chưa, có bị khóa không, để vẽ
		// bảng chọn câu mà không cần tải nội dung từng trang.
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

// GetAttemptPage trả nội dung một trang của lượt làm bài. Trang bị khóa theo luồng (TOEIC_STRICT:
// trang nghe đã qua hoặc chưa tới, trang đọc khi còn ở phần nghe) trả PAGE_LOCKED, để không
// xem trước hay quay lại được như thi thật.
func (s *service) GetAttemptPage(ctx context.Context, userID string, attemptID uint, index int) (*dto.AttemptPageResponse, *httpx.HTTPError) {
	var response *dto.AttemptPageResponse
	httpErr := s.withFlowAttempt(ctx, userID, attemptID, func(repo repository.Repository, attempt *model.Attempt, now time.Time) error {
		// Trang phải tồn tại và đang được mở theo luồng (kiểm sau lazy advance, nên đúng với giờ thật).
		if index < 0 || index >= len(attempt.Layout) {
			return ErrAttemptPageNotFound
		}
		if !pageAccess(attempt, index) {
			return ErrAttemptPageLocked
		}
		page := attempt.Layout[index]

		// Chỉ tải câu và group của trang này, không tải cả đề.
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

		// Đáp án đã chọn để client hiện lại lựa chọn khi mở lại trang.
		selected := make(map[uint]*uint, len(attempt.Answers))
		for _, answer := range attempt.Answers {
			selected[answer.QuestionID] = answer.SelectedOptionID
		}
		// Part 1, 2 như đề thật: nội dung câu hỏi và đáp án chỉ có trong audio, màn hình chỉ hiện
		// nhãn A/B/C. Ẩn được luôn vì API này chỉ phục vụ lượt đang làm; xem lại thì qua history.
		hideContent := page.Part != nil && (*page.Part == model.Part1 || *page.Part == model.Part2)

		response = &dto.AttemptPageResponse{
			Index:     page.Index,
			Section:   string(page.Section),
			Part:      dto.PartString(page.Part),
			Group:     dto.ToAttemptPageGroup(group),
			Questions: dto.ToAttemptPageQuestions(questions, numbers, selected, hideContent),
		}
		// Trang nghe đang chạy: gửi thời gian đã trôi của trang (để reload phát audio từ đúng chỗ),
		// thời gian trả lời sau audio, mốc sớm nhất được tiến trang, và audio của trang sau để tải trước.
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

// listeningAdvanceTolerance: client được tiến trang sớm hơn lúc hết audio tối đa 2s, bù lệch
// đồng hồ và độ trễ phát audio giữa client và server.
const listeningAdvanceTolerance = 2 * time.Second

// nextAudioURL trả URL audio của trang nghe kế tiếp, để client tải trước và chuyển trang không bị
// khựng. Bỏ qua các trang cùng group với trang hiện tại (part 1, 2 tách mỗi câu một trang nhưng
// có thể dùng chung audio). Hết phần nghe hoặc trang sau không có group thì trả nil.
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

// layoutNumbers tính số thứ tự hiển thị ("Câu N") cho mọi câu trong layout: dùng số câu gốc của
// đề nếu có, không thì đánh số theo thứ tự trong bài.
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

// answeredQuestions là tập câu đã chọn đáp án (bỏ qua dòng đã bỏ chọn), để đếm số câu đã làm.
func answeredQuestions(attempt *model.Attempt) map[uint]struct{} {
	answered := make(map[uint]struct{}, len(attempt.Answers))
	for _, answer := range attempt.Answers {
		if answer.SelectedOptionID != nil {
			answered[answer.QuestionID] = struct{}{}
		}
	}
	return answered
}
