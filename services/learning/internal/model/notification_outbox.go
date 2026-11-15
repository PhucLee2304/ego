package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NotificationOutbox là hàng đợi thông báo chờ gửi sang service notifications. Bản ghi được
// tạo cùng transaction với thay đổi nghiệp vụ, worker gửi sau; gửi lỗi thì thử lại, nên
// thông báo không bị mất khi notifications tạm thời không gọi được.
//   - EventKey: khóa duy nhất của sự kiện (vd "attempt:12:submitted"), chống tạo trùng
//     và gửi kèm làm idempotency key để notifications không tạo thông báo hai lần.
//   - Source: domain sinh ra thông báo.
//   - Recipients: danh sách user ID nhận thông báo.
//   - Title, Body: nội dung hiển thị.
//   - Data: dữ liệu để client mở đúng màn khi bấm (type, id, classroomId).
//   - Attempts: số lần gửi lỗi; lần thử sau cách 2^Attempts giây, tối đa 5 phút.
//   - NextAttemptAt: thời điểm sớm nhất được gửi (lần đầu, thử lại, hoặc hẹn giờ như nhắc
//     hạn nộp). Worker lấy bản ghi nào sẽ đẩy mốc này lên 1 phút để worker khác không lấy trùng.
//   - ProcessedAt: lúc gửi thành công; nil là còn chờ gửi.
//   - LastError: lỗi của lần gửi gần nhất.
type NotificationOutbox struct {
	gorm.Model
	EventKey      string                      `gorm:"type:text;not null;uniqueIndex"`
	Source        NotificationSource          `gorm:"type:text;not null"`
	Recipients    datatypes.JSONSlice[string] `gorm:"type:jsonb;not null"`
	Title         string                      `gorm:"type:text;not null"`
	Body          string                      `gorm:"type:text;not null"`
	Data          datatypes.JSONMap           `gorm:"type:jsonb;not null;default:'{}'"`
	Attempts      int                         `gorm:"not null;default:0"`
	NextAttemptAt time.Time                   `gorm:"not null;index"`
	ProcessedAt   *time.Time                  `gorm:"index"`
	LastError     *string                     `gorm:"type:text"`
}

// NotificationSource là domain đã sinh ra thông báo, gửi sang notifications làm SourceService.
//   - EXAM: điểm bị tính lại sau khi giáo viên đính chính đáp án.
//   - ATTEMPT: có kết quả bài nộp, bài bị đình chỉ hoặc được khôi phục kết quả.
//   - MEMBER: có yêu cầu vào lớp, yêu cầu được duyệt/từ chối, bị xóa khỏi lớp.
//   - ROOM: buổi học trực tuyến đã bắt đầu.
//   - ASSIGNMENT: có bài tập mới, bài tập sắp hết hạn.
//   - CLASSROOM, SCHEDULE: dành sẵn, hiện chưa có chỗ nào gửi.
type NotificationSource string

const (
	NotificationSourceExam       NotificationSource = "EXAM"
	NotificationSourceAttempt    NotificationSource = "ATTEMPT"
	NotificationSourceClassroom  NotificationSource = "CLASSROOM"
	NotificationSourceMember     NotificationSource = "MEMBER"
	NotificationSourceSchedule   NotificationSource = "SCHEDULE"
	NotificationSourceRoom       NotificationSource = "ROOM"
	NotificationSourceAssignment NotificationSource = "ASSIGNMENT"
)
