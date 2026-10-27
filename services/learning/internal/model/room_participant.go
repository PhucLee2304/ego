package model

import (
	"time"

	"gorm.io/gorm"
)

type RoomParticipant struct {
	gorm.Model
	RoomID uint `gorm:"not null;index;index:idx_room_participants_history,priority:1"`
	Room   Room `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`

	UserID   string              `gorm:"type:text;not null;index;index:idx_room_participants_history,priority:2"`
	Role     RoomParticipantRole `gorm:"type:text;not null;index"`
	JoinedAt time.Time           `gorm:"not null;index;index:idx_room_participants_history,priority:3"`
	LeftAt   *time.Time          `gorm:"index"`
}

type RoomParticipantRole string

const (
	RoomParticipantRoleHost        RoomParticipantRole = "HOST"
	RoomParticipantRoleParticipant RoomParticipantRole = "PARTICIPANT"
)
