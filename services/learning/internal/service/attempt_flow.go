package service

import (
	"time"

	"ego/services/learning/internal/model"
)

// Các hàm thuần của luồng TOEIC_STRICT (tiến trang nghe, khóa trang): chỉ đọc/ghi struct
// attempt, không chạm DB, để test không cần DB.

// defaultListeningGrace là thời gian dư mặc định cộng vào hạn mỗi trang nghe, bù trễ mạng và
// tải audio để học viên không mất trang vì mạng chậm.
const defaultListeningGrace = 20 * time.Second

// listeningGrace lấy thời gian dư từ config; không có config hoặc giá trị <= 0 thì dùng mặc định.
func (s *service) listeningGrace() time.Duration {
	if s.cfg == nil || s.cfg.TOEICListeningGraceMs <= 0 {
		return defaultListeningGrace
	}
	return time.Duration(s.cfg.TOEICListeningGraceMs) * time.Millisecond
}

// isStrict cho biết lượt làm bài theo luồng TOEIC_STRICT (như thi thật).
func isStrict(attempt *model.Attempt) bool {
	return attempt.Flow == model.AttemptFlowTOEICStrict
}

// inListening cho biết lượt TOEIC_STRICT còn đang ở phần nghe, nơi trang tự chuyển theo giờ.
func inListening(attempt *model.Attempt) bool {
	return isStrict(attempt) && attempt.Stage != nil && *attempt.Stage == model.AttemptStageListening
}

// listeningPageCount đếm số trang nghe của layout. Layout sắp theo thứ tự section nên trang nghe
// luôn là đoạn đầu; nhờ vậy chỉ số trang nghe (ListeningPage) cũng chính là chỉ số trong layout,
// và gặp trang đầu tiên không phải nghe là dừng đếm được.
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

// listeningDeadline là thời điểm server tự chuyển khỏi một trang nghe: lúc trang bắt đầu, cộng độ
// dài audio, cộng thời gian trả lời sau audio, cộng thời gian dư bù trễ mạng và tải audio.
func listeningDeadline(page model.AttemptPage, startedAt time.Time, grace time.Duration) time.Time {
	return startedAt.Add(time.Duration(page.AudioMs+page.GapMs)*time.Millisecond + grace)
}

// advanceListening đưa con trỏ trang nghe về đúng vị trí theo giờ hiện tại (lazy advance). Server
// không chạy đồng hồ nền cho từng bài, nên hàm này được gọi mỗi khi có request chạm vào attempt.
// Chỉ sửa struct, caller lưu DB nếu hàm trả true (có thay đổi).
func advanceListening(attempt *model.Attempt, now time.Time, grace time.Duration) bool {
	// Không còn ở phần nghe (hoặc không phải TOEIC_STRICT) thì không có gì để tiến.
	if !inListening(attempt) {
		return false
	}

	// Mốc bắt đầu của trang nghe hiện tại; attempt cũ chưa có thì lấy lúc bắt đầu làm bài.
	count := listeningPageCount(attempt.Layout)
	changed := false
	startedAt := attempt.StartedAt
	if attempt.ListeningPageStartedAt != nil {
		startedAt = *attempt.ListeningPageStartedAt
	}
	// Có thể vượt nhiều trang một lần (vd mất mạng hay đóng tab vài phút): mỗi trang đã quá hạn
	// thì sang trang sau. Trang mới bắt đầu đúng lúc trang trước hết hạn, không phải lúc gọi hàm,
	// để thời gian các trang không bị kéo dài theo lúc request tới.
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
	// Qua hết trang nghe thì chuyển sang phần đọc.
	if attempt.ListeningPage >= count {
		stage := model.AttemptStageReading
		attempt.Stage = &stage
		changed = true
	}
	return changed
}

// listeningOnlyFinished cho biết một lượt TOEIC_STRICT chỉ gồm trang nghe (vd chỉ luyện Part 1)
// đã nghe hết. Lượt như vậy sang READING là không còn trang nào để làm, nên server nộp bài ngay,
// như khi hết giờ, thay vì để học viên kẹt ở màn trống.
func listeningOnlyFinished(attempt *model.Attempt) bool {
	return isStrict(attempt) && !inListening(attempt) && len(attempt.Layout) > 0 &&
		listeningPageCount(attempt.Layout) == len(attempt.Layout)
}

// pageAccess cho biết trang index có được mở không, là luật khóa trang dùng chung cho API lấy
// trang, bảng chọn câu và lưu đáp án:
//   - FREE: mở mọi trang.
//   - TOEIC_STRICT đang nghe: chỉ mở đúng trang nghe hiện tại; trang nghe đã qua, chưa tới và mọi
//     trang đọc đều khóa.
//   - TOEIC_STRICT đã sang đọc: mở mọi trang đọc, khóa toàn bộ trang nghe.
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

// pageOfQuestion tìm trang chứa câu trong layout của attempt; -1 nghĩa là câu không thuộc lượt
// làm bài (ngoài phạm vi section/part đã chọn).
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

// questionAccess cho biết câu có được chọn đáp án không: câu phải thuộc lượt làm bài và trang
// chứa nó đang mở. Dùng khi lưu đáp án qua WebSocket và khi lọc body lúc nộp bài, để ở
// TOEIC_STRICT không sửa được đáp án của trang đã khóa bằng cả hai đường.
func questionAccess(attempt *model.Attempt, questionID uint) bool {
	index := pageOfQuestion(attempt, questionID)
	return index >= 0 && pageAccess(attempt, index)
}
