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

	ContextType AttemptContextType `gorm:"type:text;not null;default:STANDALONE;index"`
	ContextID   *uint              `gorm:"index"`

	// Luồng làm bài, chốt lúc tạo attempt (PLAN_TOEIC_ATTEMPT.md mục 1.1–1.3).
	Flow                   AttemptFlow                      `gorm:"type:text;not null;default:FREE"`
	Stage                  *AttemptStage                    `gorm:"type:text"`
	Layout                 datatypes.JSONSlice[AttemptPage] `gorm:"type:jsonb"`
	ListeningPage          int                              `gorm:"not null;default:0"`
	ListeningPageStartedAt *time.Time

	// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 1.2–1.8). Policy chụp lúc tạo
	// attempt, sửa assignment sau đó không ảnh hưởng.
	Proctored               bool                                         `gorm:"not null;default:false"`
	IntegrityPolicy         *datatypes.JSONType[IntegrityPolicySnapshot] `gorm:"type:jsonb"`
	IntegrityConsentAt      *time.Time
	IntegrityConsentVersion *string `gorm:"type:text"`
	ViolationCount          int     `gorm:"not null;default:0"`
	SessionTokenHash        *string `gorm:"type:text"`
	SessionEpoch            int     `gorm:"not null;default:0"`
	SessionLastSeenAt       *time.Time

	// RawScore là điểm chấm từ đáp án; Score là điểm cuối, chỉ ghi qua applyScore.
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
	// STANDALONE means the attempt is created directly from Exams, not from another module.
	AttemptContextTypeStandalone AttemptContextType = "STANDALONE"

	// CLASSROOM_ASSIGNMENT means the attempt belongs to a classroom assignment; ContextID is the assignment ID.
	AttemptContextTypeClassroomAssignment AttemptContextType = "CLASSROOM_ASSIGNMENT"
)

type AttemptFlow string

const (
	// FREE: làm tự do, áp cho THPT và assignment.
	AttemptFlowFree AttemptFlow = "FREE"

	// TOEIC_STRICT: làm theo trang, phần nghe tuần tự, phần đọc tự do.
	AttemptFlowTOEICStrict AttemptFlow = "TOEIC_STRICT"
)

type AttemptStage string

const (
	AttemptStageListening AttemptStage = "LISTENING"
	AttemptStageReading   AttemptStage = "READING"
)
