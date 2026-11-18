package model

import (
	"time"

	"gorm.io/gorm"
)

// AttemptIntegrityEvent là một sự kiện giám sát của attempt, do client gửi lên (rời màn,
// copy/paste…) hoặc server tự ghi (chiếm phiên, giáo viên xử lý…).
//   - EventID: ID do bên tạo sinh, unique theo attempt để gửi lại không bị ghi trùng.
//   - SessionEpoch, ConnectionID: phiên và kết nối WebSocket đã gửi sự kiện.
//   - Type, Level: loại sự kiện và mức độ (LOG, WARNING, VIOLATION).
//   - ViolationNo: số thứ tự vi phạm trong attempt, chỉ có khi Level = VIOLATION.
//   - StartEventID: với AWAY_END, trỏ về EventID của AWAY_START cùng lần rời màn.
//   - ServerDurationMs, ClientDurationMs: thời gian rời màn đo bởi server và client.
//   - Replayed: client gửi bù sau khi mất kết nối, không quan sát trực tiếp được.
//   - Stale: gửi từ kết nối không phải chủ phiên hoặc phiên cũ.
//   - Detail: ghi chú thêm (vd ghi chú của giáo viên).
//   - ClientAt, ReceivedAt: giờ theo client và giờ server nhận.
//   - Review, ReviewedAt, ReviewedBy: kết quả giáo viên xem xét thẻ vi phạm.
type AttemptIntegrityEvent struct {
	gorm.Model
	AttemptID    uint               `gorm:"not null;index"`
	EventID      string             `gorm:"type:text;not null"`
	SessionEpoch int                `gorm:"not null;default:0"`
	ConnectionID *string            `gorm:"type:text"`
	Type         IntegrityEventType `gorm:"type:text;not null"`
	Level        IntegrityLevel     `gorm:"type:text;not null;index"`
	ViolationNo  *int
	StartEventID *string `gorm:"type:text"`

	ServerDurationMs *int
	ClientDurationMs *int
	Replayed         bool    `gorm:"not null;default:false"`
	Stale            bool    `gorm:"not null;default:false"`
	Detail           *string `gorm:"type:text"`

	ClientAt   *time.Time
	ReceivedAt time.Time `gorm:"not null"`

	Review     *IntegrityReview `gorm:"type:text"`
	ReviewedAt *time.Time
	ReviewedBy *string `gorm:"type:text"`
}

// IntegrityPolicySnapshot là cấu hình giám sát chụp vào attempt lúc tạo, để sửa assignment
// hoặc config sau đó không ảnh hưởng bài đang làm: số vi phạm được phép, thời gian chờ
// trước khi phiên khác được chiếm quyền (ms) và phiên bản điều khoản học viên đã đồng ý.
type IntegrityPolicySnapshot struct {
	MaxViolations   int    `json:"maxViolations"`
	TakeoverDelayMs int    `json:"takeoverDelayMs"`
	ConsentVersion  string `json:"consentVersion"`
}

// IntegrityOutcome là kết quả giám sát của attempt: NONE (bình thường), TERMINATED (bị
// đình chỉ, Score = 0) hoặc REINSTATED (giáo viên khôi phục, Score lấy lại từ RawScore).
type IntegrityOutcome string

const (
	IntegrityOutcomeNone       IntegrityOutcome = "NONE"
	IntegrityOutcomeTerminated IntegrityOutcome = "TERMINATED"
	IntegrityOutcomeReinstated IntegrityOutcome = "REINSTATED"
)

// IntegrityTerminationReason là lý do đình chỉ: MAX_VIOLATIONS (vượt số vi phạm được
// phép, hệ thống tự đình chỉ) hoặc TEACHER_BANNED (giáo viên cấm từ thẻ vi phạm).
type IntegrityTerminationReason string

const (
	IntegrityTerminationMaxViolations IntegrityTerminationReason = "MAX_VIOLATIONS"
	IntegrityTerminationTeacherBanned IntegrityTerminationReason = "TEACHER_BANNED"
)

// IntegrityLevel là mức độ của sự kiện: LOG (chỉ lưu vết), WARNING (đáng chú ý nhưng không
// tính vi phạm, vd rời màn gửi bù) hoặc VIOLATION (tính vào ViolationCount).
type IntegrityLevel string

const (
	IntegrityLevelLog       IntegrityLevel = "LOG"
	IntegrityLevelWarning   IntegrityLevel = "WARNING"
	IntegrityLevelViolation IntegrityLevel = "VIOLATION"
)

// IntegrityEventType là loại sự kiện giám sát.
//   - Client gửi: AWAY_START/AWAY_END (rời và quay lại màn hình), RELOAD (tải lại trang),
//     NAVIGATION_BLOCKED/BACK_BLOCKED (bị chặn rời trang/bấm back), COPY, PASTE.
//   - Server ghi về phiên: SESSION_CONFLICT (phiên khác đang giữ quyền), SESSION_TAKEOVER
//     (phiên mới chiếm quyền).
//   - Server ghi khi giáo viên xử lý: SESSION_RELEASED (giải phóng phiên khi học viên bị kẹt),
//     TEACHER_BANNED, CONTINUE_ALLOWED, VIOLATION_KEPT, RESULT_REINSTATED (khôi phục kết quả).
type IntegrityEventType string

const (
	IntegrityEventAwayStart         IntegrityEventType = "AWAY_START"
	IntegrityEventAwayEnd           IntegrityEventType = "AWAY_END"
	IntegrityEventReload            IntegrityEventType = "RELOAD"
	IntegrityEventNavigationBlocked IntegrityEventType = "NAVIGATION_BLOCKED"
	IntegrityEventBackBlocked       IntegrityEventType = "BACK_BLOCKED"
	IntegrityEventCopy              IntegrityEventType = "COPY"
	IntegrityEventPaste             IntegrityEventType = "PASTE"
	IntegrityEventSessionConflict   IntegrityEventType = "SESSION_CONFLICT"
	IntegrityEventSessionTakeover   IntegrityEventType = "SESSION_TAKEOVER"
	IntegrityEventSessionReleased   IntegrityEventType = "SESSION_RELEASED"
	IntegrityEventTeacherBanned     IntegrityEventType = "TEACHER_BANNED"
	IntegrityEventContinueAllowed   IntegrityEventType = "CONTINUE_ALLOWED"
	IntegrityEventViolationKept     IntegrityEventType = "VIOLATION_KEPT"
	IntegrityEventResultReinstated  IntegrityEventType = "RESULT_REINSTATED"
)

// IntegrityReview là quyết định của giáo viên trên một thẻ vi phạm:
//   - CONTINUE_ALLOWED: cho làm tiếp; bài còn đang làm thì ViolationCount giảm 1.
//   - BANNED: cấm, đình chỉ attempt (TEACHER_BANNED).
//   - KEPT: đã xem và vẫn tính vi phạm (không trừ, không cấm).
type IntegrityReview string

const (
	IntegrityReviewContinueAllowed IntegrityReview = "CONTINUE_ALLOWED"
	IntegrityReviewBanned          IntegrityReview = "BANNED"
	IntegrityReviewKept            IntegrityReview = "KEPT"
)

// AttemptIntegrityEventRow là event kèm học sinh của attempt (kết quả join, không phải bảng).
type AttemptIntegrityEventRow struct {
	AttemptIntegrityEvent `gorm:"embedded"`
	StudentID             string
}

// AttemptIntegritySummary là tóm tắt giám sát của một attempt cho gradebook;
// PendingViolations là số vi phạm giáo viên chưa xem xét.
type AttemptIntegritySummary struct {
	AttemptID         uint
	Proctored         bool
	ViolationCount    int
	PendingViolations int
	IntegrityOutcome  IntegrityOutcome
	TerminationReason *IntegrityTerminationReason
}
