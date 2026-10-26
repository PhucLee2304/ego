package service

import (
	"time"

	"ego/services/learning/internal/model"
)

// Các hàm thuần của luồng TOEIC_STRICT (PLAN_TOEIC_ATTEMPT.md mục 1.3–1.4): chỉ đọc/ghi
// struct attempt, không chạm DB, để test không cần DB.

const defaultListeningGrace = 20 * time.Second

func (s *service) listeningGrace() time.Duration {
	if s.cfg == nil || s.cfg.TOEICListeningGraceMs <= 0 {
		return defaultListeningGrace
	}
	return time.Duration(s.cfg.TOEICListeningGraceMs) * time.Millisecond
}

func isStrict(attempt *model.Attempt) bool {
	return attempt.Flow == model.AttemptFlowTOEICStrict
}

func inListening(attempt *model.Attempt) bool {
	return isStrict(attempt) && attempt.Stage != nil && *attempt.Stage == model.AttemptStageListening
}

// listeningPageCount đếm trang nghe. Layout sắp theo section.order nên trang nghe luôn
// là đoạn đầu của layout; ListeningPage là chỉ số trong layout.
func listeningPageCount(layout []model.AttemptPage) int {
	count := 0
	for _, page := range layout {
		if page.Section != model.SectionCodeListening {
			break
		}
		count++
	}
	return count
}

// listeningDeadline là mốc server tự tiến khỏi trang nghe hiện tại.
func listeningDeadline(page model.AttemptPage, startedAt time.Time, grace time.Duration) time.Time {
	return startedAt.Add(time.Duration(page.AudioMs+page.GapMs)*time.Millisecond + grace)
}

// advanceListening tiến con trỏ nghe khi đã quá startedAt + audioMs + gapMs + grace,
// có thể nhiều trang một lần; qua trang nghe cuối thì Stage = READING. Trả true nếu đổi.
func advanceListening(attempt *model.Attempt, now time.Time, grace time.Duration) bool {
	if !inListening(attempt) {
		return false
	}

	count := listeningPageCount(attempt.Layout)
	changed := false
	startedAt := attempt.StartedAt
	if attempt.ListeningPageStartedAt != nil {
		startedAt = *attempt.ListeningPageStartedAt
	}
	for attempt.ListeningPage < count {
		deadline := listeningDeadline(attempt.Layout[attempt.ListeningPage], startedAt, grace)
		if !now.After(deadline) {
			break
		}
		attempt.ListeningPage++
		startedAt = deadline
		changed = true
	}
	if changed {
		attempt.ListeningPageStartedAt = &startedAt
	}
	if attempt.ListeningPage >= count {
		stage := model.AttemptStageReading
		attempt.Stage = &stage
		changed = true
	}
	return changed
}

// pageAccess cho biết trang index có mở được không (bảng ở mục 1.4). Flow FREE mở mọi trang.
func pageAccess(attempt *model.Attempt, index int) bool {
	if index < 0 || index >= len(attempt.Layout) {
		return false
	}
	if !isStrict(attempt) {
		return true
	}
	if attempt.Layout[index].Section == model.SectionCodeListening {
		return inListening(attempt) && index == attempt.ListeningPage
	}
	return !inListening(attempt)
}

// pageOfQuestion trả chỉ số trang chứa câu, -1 nếu câu không thuộc attempt.
func pageOfQuestion(attempt *model.Attempt, questionID uint) int {
	for _, page := range attempt.Layout {
		for _, id := range page.QuestionIDs {
			if id == questionID {
				return page.Index
			}
		}
	}
	return -1
}

// questionAccess cho biết câu có chọn đáp án được không: câu thuộc attempt và trang của nó đang mở.
func questionAccess(attempt *model.Attempt, questionID uint) bool {
	index := pageOfQuestion(attempt, questionID)
	return index >= 0 && pageAccess(attempt, index)
}
