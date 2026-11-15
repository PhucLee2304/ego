package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Attempt struct {
	gorm.Model
	Mode           AttemptMode                   `gorm:"type:text;not null;index"`
	Status         AttemptStatus                 `gorm:"type:text;not null;index"`
	SectionCode    *SectionCode                  `gorm:"type:text;index"`
	PartCodes      datatypes.JSONSlice[PartCode] `gorm:"type:jsonb"`
	StartedAt      time.Time                     `gorm:"not null"`
	ExpiresAt      *time.Time                    `gorm:"index"`
	SubmittedAt    *time.Time
	Duration       *int
	TotalQuestions int `gorm:"not null;default:0"`
	CorrectAnswers *int
	Score          *float64 `gorm:"type:numeric(5,2)"`

	ExamID uint `gorm:"not null;index"`
	Exam   Exam `gorm:"foreignKey:ExamID;references:ID"`

	UserID string `gorm:"type:text;not null;index"`

	// Nguồn của attempt:
	//   - ContextType: attempt sinh ra từ đâu (STANDALONE: tự làm đề; CLASSROOM_ASSIGNMENT: bài tập lớp).
	//   - ContextID: ID bản ghi tương ứng với ContextType (assignment ID khi CLASSROOM_ASSIGNMENT).
	//     Không có khóa ngoại vì cột này trỏ tới bảng khác nhau tùy ContextType.
	ContextType AttemptContextType `gorm:"type:text;not null;default:STANDALONE;index"`
	ContextID   *uint              `gorm:"index"`

	// Luồng làm bài (Flow và Layout chốt lúc tạo attempt, không đổi về sau):
	//   - Flow: FREE (làm tự do, đi trang nào cũng được) hoặc TOEIC_STRICT (như thi thật).
	//   - Stage: chỉ có với TOEIC_STRICT; LISTENING khi đang ở phần nghe, qua hết trang nghe
	//     thì sang READING. Đề không có phần nghe thì bắt đầu luôn ở READING. nil với FREE.
	//   - Layout: danh sách trang của bài, dựng một lần từ đề đã lọc theo section/part. Mỗi
	//     trang gồm các câu hỏi, độ dài audio và thời gian trả lời sau audio.
	//   - ListeningPage: chỉ số trang nghe hiện tại trong Layout; server tự tăng khi hết giờ
	//     của trang (audio + thời gian trả lời + thời gian dư), không lùi lại được.
	//   - ListeningPageStartedAt: thời điểm trang nghe hiện tại bắt đầu, làm mốc tính hạn trang.
	Flow                   AttemptFlow                      `gorm:"type:text;not null;default:FREE"`
	Stage                  *AttemptStage                    `gorm:"type:text"`
	Layout                 datatypes.JSONSlice[AttemptPage] `gorm:"type:jsonb"`
	ListeningPage          int                              `gorm:"not null;default:0"`
	ListeningPageStartedAt *time.Time

	// Giám sát làm bài (chỉ dùng khi Proctored = true):
	//   - Proctored: attempt có bị giám sát không, lấy từ assignment lúc tạo.
	//   - IntegrityPolicy: bản chụp cấu hình giám sát lúc tạo attempt (số vi phạm tối đa,
	//     thời gian chờ chiếm phiên, phiên bản điều khoản); sửa assignment sau đó không ảnh hưởng.
	//   - IntegrityConsentAt, IntegrityConsentVersion: lúc học viên đồng ý điều khoản và bản đã đồng ý.
	//   - ViolationCount: số lần vi phạm đã ghi nhận; vượt MaxViolations thì bị đình chỉ.
	//   - SessionTokenHash: hash token của phiên (thiết bị/tab) đang giữ quyền làm bài.
	//   - SessionEpoch: tăng mỗi khi phiên mới chiếm quyền; phiên cũ mang epoch cũ bị từ chối.
	//   - SessionLastSeenAt: lần cuối phiên hiện tại còn kết nối, mốc tính thời gian chờ chiếm phiên.
	Proctored               bool                                         `gorm:"not null;default:false"`
	IntegrityPolicy         *datatypes.JSONType[IntegrityPolicySnapshot] `gorm:"type:jsonb"`
	IntegrityConsentAt      *time.Time
	IntegrityConsentVersion *string `gorm:"type:text"`
	ViolationCount          int     `gorm:"not null;default:0"`
	SessionTokenHash        *string `gorm:"type:text"`
	SessionEpoch            int     `gorm:"not null;default:0"`
	SessionLastSeenAt       *time.Time

	// Kết quả và đình chỉ:
	//   - RawScore: điểm chấm từ đáp án; Score là điểm cuối, chỉ ghi qua applyScore
	//     (bị đình chỉ thì Score = 0, RawScore vẫn giữ để khôi phục).
	//   - IntegrityOutcome: NONE (bình thường), TERMINATED (bị đình chỉ) hoặc REINSTATED
	//     (giáo viên đã khôi phục, Score lấy lại bằng RawScore).
	//   - TerminatedAt, TerminationReason, TerminatedBy: lúc đình chỉ, lý do (MAX_VIOLATIONS:
	//     vượt số vi phạm; TEACHER_BANNED: giáo viên đình chỉ) và người đình chỉ
	//     ("system" hoặc ID giáo viên).
	//   - ReinstatedAt, ReinstatedBy, ReinstateNote: lúc khôi phục, giáo viên khôi phục và ghi chú.
	RawScore          *float64         `gorm:"type:numeric(5,2)"`
	IntegrityOutcome  IntegrityOutcome `gorm:"type:text;not null;default:NONE"`
	TerminatedAt      *time.Time
	TerminationReason *IntegrityTerminationReason `gorm:"type:text"`
	TerminatedBy      *string                     `gorm:"type:text"`
	ReinstatedAt      *time.Time
	ReinstatedBy      *string `gorm:"type:text"`
	ReinstateNote     *string `gorm:"type:text"`

	Answers []AttemptAnswer `gorm:"foreignKey:AttemptID;references:ID"`
	History *History        `gorm:"foreignKey:AttemptID;references:ID"`
}

type AttemptMode string

const (
	AttemptModePractice   AttemptMode = "PRACTICE"
	AttemptModeTest       AttemptMode = "TEST"
	AttemptModeAssignment AttemptMode = "ASSIGNMENT"
)

type AttemptStatus string

const (
	AttemptStatusInProgress AttemptStatus = "ACTIVE"
	AttemptStatusSubmitted  AttemptStatus = "SUBMITTED"
	AttemptStatusCancelled  AttemptStatus = "CANCELLED"
)

type AttemptContextType string

const (
	// Attempt học viên tự làm từ danh sách đề, không gắn với lớp học nào; ContextID luôn nil.
	AttemptContextTypeStandalone AttemptContextType = "STANDALONE"

	// Attempt làm bài tập được giao trong lớp học; ContextID là ID của assignment.
	AttemptContextTypeClassroomAssignment AttemptContextType = "CLASSROOM_ASSIGNMENT"
)

type AttemptFlow string

const (
	// FREE: làm tự do, áp dụng với THPT và assignment.
	AttemptFlowFree AttemptFlow = "FREE"

	// TOEIC_STRICT: làm theo trang, phần nghe tuần tự, phần đọc tự do.
	AttemptFlowTOEICStrict AttemptFlow = "TOEIC_STRICT"
)

type AttemptStage string

const (
	AttemptStageListening AttemptStage = "LISTENING"
	AttemptStageReading   AttemptStage = "READING"
)
