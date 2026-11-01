package model

import (
	"time"

	"gorm.io/gorm"
)

type RoomParticipantState struct {
	gorm.Model
	UserID string `gorm:"type:text;not null;uniqueIndex:idx_room_participant_states_room_user"`

	RoomID uint `gorm:"not null;uniqueIndex:idx_room_participant_states_room_user"`
	Room   Room `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`

	MicrophoneAllowed bool       `gorm:"not null;default:true"`
	CameraAllowed     bool       `gorm:"not null;default:true"`
	HandRaisedAt      *time.Time `gorm:"index"`
	RemovedAt         *time.Time `gorm:"index"`
	// RemovedReason: lý do giáo viên ghi khi xóa, chỉ học sinh đó và giáo viên thấy.
	RemovedReason          *string    `gorm:"type:text"`
	ScreenShareRequestedAt *time.Time `gorm:"index"`
	ScreenShareApprovedAt  *time.Time `gorm:"index"`
}
