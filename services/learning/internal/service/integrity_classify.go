package service

import (
	"time"

	"ego/services/learning/internal/model"
)

// integrityConfig là các ngưỡng giám sát (PLAN_ATTEMPT_INTEGRITY.md mục 1.4).
type integrityConfig struct {
	// TakeoverDelay: phiên cũ phải ngắt quá thời gian này mới cho chiếm.
	TakeoverDelay time.Duration
}

var defaultIntegrityConfig = integrityConfig{
	TakeoverDelay: 30 * time.Second,
}

// classifyAwayStart: rời màn là vi phạm ngay khi server nhận AWAY_START (mục 1.3),
// không tính thời lượng. Chỉ không tính khi không quan sát trực tiếp được:
// từ kết nối không phải chủ phiên / thế hệ phiên cũ (LOG), hoặc gửi bù (WARNING).
// Mỗi AWAY_START có eventId riêng nên một lần rời màn chỉ được tính một lần.
func classifyAwayStart(start *model.AttemptIntegrityEvent, attempt *model.Attempt) model.IntegrityLevel {
	if start.Stale || start.SessionEpoch != attempt.SessionEpoch {
		return model.IntegrityLevelLog
	}
	if start.Replayed {
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
