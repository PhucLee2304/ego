package service

import (
	"time"

	"ego/services/learning/internal/model"
)

// integrityConfig là các ngưỡng giám sát (PLAN_ATTEMPT_INTEGRITY.md mục 1.3–1.4).
type integrityConfig struct {
	// AwayGrace: thời gian rời màn tối thiểu (đo phía server) để tính vi phạm.
	AwayGrace time.Duration
	// Grace: ân hạn sau khi tạo attempt hoặc claim phiên (F5/reconnect).
	Grace time.Duration
	// TakeoverDelay: phiên cũ phải ngắt quá thời gian này mới cho chiếm.
	TakeoverDelay time.Duration
}

var defaultIntegrityConfig = integrityConfig{
	AwayGrace:     3 * time.Second,
	Grace:         10 * time.Second,
	TakeoverDelay: 30 * time.Second,
}

// classifyEpisode quyết định mức của một episode rời màn từ cặp AWAY_START/AWAY_END
// đã lưu (mục 1.3). Điều kiện "chưa từng được tính" do unique index bảo đảm, không
// xét ở đây. Thời gian client gửi không bao giờ được dùng.
func classifyEpisode(start, end *model.AttemptIntegrityEvent, attempt *model.Attempt, cfg integrityConfig) model.IntegrityLevel {
	// Từ kết nối không phải chủ phiên hoặc thế hệ phiên đã thu hồi.
	if start.Stale || end.Stale || start.SessionEpoch != attempt.SessionEpoch || end.SessionEpoch != attempt.SessionEpoch {
		return model.IntegrityLevelLog
	}
	// Không quan sát trọn vẹn: gửi bù, hoặc mất kết nối giữa chừng.
	if start.Replayed || end.Replayed || start.ConnectionID == nil || end.ConnectionID == nil || *start.ConnectionID != *end.ConnectionID {
		return model.IntegrityLevelWarning
	}
	away := end.ReceivedAt.Sub(start.ReceivedAt)
	if away < cfg.AwayGrace {
		return model.IntegrityLevelLog
	}
	// Ân hạn chỉ trừ phần thời gian nằm trong nó, không miễn cả episode.
	from := start.ReceivedAt
	if start.GraceUntil != nil && start.GraceUntil.After(from) {
		from = *start.GraceUntil
	}
	if end.ReceivedAt.Sub(from) < cfg.AwayGrace {
		return model.IntegrityLevelWarning
	}
	return model.IntegrityLevelViolation
}

// exceededMaxViolations: MaxViolations là số lần được phép, vượt quá thì đình chỉ.
func exceededMaxViolations(attempt *model.Attempt) bool {
	if !attempt.Proctored || attempt.IntegrityPolicy == nil {
		return false
	}
	return attempt.ViolationCount > attempt.IntegrityPolicy.Data().MaxViolations
}
