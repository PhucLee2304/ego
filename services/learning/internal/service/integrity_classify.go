package service

import (
	"time"

	"ego/services/learning/internal/model"
)

// integrityConfig là các ngưỡng giám sát chỉnh được qua config.
type integrityConfig struct {
	// TakeoverDelay: phiên cũ phải ngắt quá thời gian này mới cho chiếm.
	TakeoverDelay time.Duration
}

// defaultIntegrityConfig: phiên cũ phải ngắt quá 30 giây mới cho thiết bị khác chiếm, đủ để
// mạng chập chờn kịp kết nối lại mà không mất phiên.
var defaultIntegrityConfig = integrityConfig{
	TakeoverDelay: 30 * time.Second,
}

// classifyAwayStart quyết định mức độ của một lần học viên rời màn hình làm bài (sự kiện
// AWAY_START). Rời màn bị tính vi phạm ngay khi server nhận sự kiện, không cần chờ biết rời
// bao lâu, để không lách được bằng cách quay lại thật nhanh. Mỗi AWAY_START có eventId riêng
// nên một lần rời màn chỉ được tính một lần.
func classifyAwayStart(start *model.AttemptIntegrityEvent, attempt *model.Attempt) model.IntegrityLevel {
	// Gửi từ kết nối không phải chủ phiên, hoặc từ phiên cũ đã bị thay: đó không phải màn hình
	// đang làm bài, nên chỉ lưu vết (LOG), không tính vi phạm.
	if start.Stale || start.SessionEpoch != attempt.SessionEpoch {
		return model.IntegrityLevelLog
	}
	// Client gửi bù sau khi mất kết nối: server không quan sát được lúc xảy ra nên không chắc
	// chắn, chỉ cảnh báo (WARNING) cho giáo viên xem, không tính vi phạm.
	if start.Replayed {
		return model.IntegrityLevelWarning
	}
	// Server nhận trực tiếp từ đúng phiên đang làm bài: tính là vi phạm.
	return model.IntegrityLevelViolation
}

// exceededMaxViolations cho biết lượt làm bài đã vượt số vi phạm được phép, tức phải bị đình
// chỉ. MaxViolations là số lần được phép, nên chỉ đình chỉ khi ViolationCount lớn hơn hẳn
// (vd MaxViolations = 2 thì lần vi phạm thứ ba mới bị đình chỉ). Ngưỡng lấy từ bản chụp policy
// lúc tạo attempt, không đọc từ assignment hiện tại.
func exceededMaxViolations(attempt *model.Attempt) bool {
	// Bài không giám sát, hoặc không có policy, thì không bao giờ bị đình chỉ vì vi phạm.
	if !attempt.Proctored || attempt.IntegrityPolicy == nil {
		return false
	}
	return attempt.ViolationCount > attempt.IntegrityPolicy.Data().MaxViolations
}
