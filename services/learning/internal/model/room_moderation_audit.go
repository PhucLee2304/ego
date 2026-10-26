package model

import "gorm.io/gorm"

type RoomModerationAudit struct {
	gorm.Model
	RoomID uint `gorm:"not null;index"`
	Room   Room `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`

	ActorUserID  string                    `gorm:"type:text;not null;index"`
	TargetUserID string                    `gorm:"type:text;not null;index"`
	Action       RoomModerationAuditAction `gorm:"type:text;not null;index"`
	Reason       *string                   `gorm:"type:text"`
}

type RoomModerationAuditAction string

const (
	RoomModerationAuditActionMute          RoomModerationAuditAction = "MUTE"
	RoomModerationAuditActionDisableCamera RoomModerationAuditAction = "DISABLE_CAMERA"
	RoomModerationAuditActionRemove        RoomModerationAuditAction = "REMOVE"
)
