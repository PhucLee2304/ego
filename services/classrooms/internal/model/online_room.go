package model

import (
	"time"

	"gorm.io/gorm"
)

type Room struct {
	gorm.Model
	ClassroomID uint      `gorm:"not null;index"`
	Classroom   Classroom `gorm:"foreignKey:ClassroomID;references:ID"`

	EventID         *uint        `gorm:"index"`
	Event           *Event       `gorm:"foreignKey:EventID;references:ID"`
	CreatedBy       string       `gorm:"type:text;not null;index"`
	Provider        RoomProvider `gorm:"type:text;not null;index"`
	Title           string       `gorm:"type:text;not null"`
	JoinURL         *string      `gorm:"type:text"`
	HostURL         *string      `gorm:"type:text"`
	Status          RoomStatus   `gorm:"type:text;not null;default:SCHEDULED;index"`
	StartsAt        *time.Time   `gorm:"index"`
	EndsAt          *time.Time   `gorm:"index"`
	MaxParticipants int          `gorm:"not null;default:10"`

	Participants []RoomParticipant `gorm:"foreignKey:RoomID;references:ID"`
}

type RoomProvider string

const (
	RoomProviderExternalURL    RoomProvider = "EXTERNAL_URL"
	RoomProviderJitsi          RoomProvider = "JITSI"
	RoomProviderInternalWebRTC RoomProvider = "INTERNAL_WEBRTC"
)

type RoomStatus string

const (
	RoomStatusScheduled RoomStatus = "SCHEDULED"
	RoomStatusLive      RoomStatus = "LIVE"
	RoomStatusEnded     RoomStatus = "ENDED"
)

type RoomParticipant struct {
	gorm.Model
	RoomID uint `gorm:"not null;index;uniqueIndex:idx_room_participants_room_user"`
	Room   Room `gorm:"foreignKey:RoomID;references:ID"`

	UserID     string              `gorm:"type:text;not null;index;uniqueIndex:idx_room_participants_room_user"`
	Role       RoomParticipantRole `gorm:"type:text;not null;index"`
	JoinedAt   *time.Time          `gorm:"index"`
	LeftAt     *time.Time          `gorm:"index"`
	LastSeenAt *time.Time          `gorm:"index"`
}

type RoomParticipantRole string

const (
	RoomParticipantRoleHost        RoomParticipantRole = "HOST"
	RoomParticipantRoleCoHost      RoomParticipantRole = "CO_HOST"
	RoomParticipantRoleParticipant RoomParticipantRole = "PARTICIPANT"
)
