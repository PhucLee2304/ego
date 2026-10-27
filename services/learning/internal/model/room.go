package model

import (
	"time"

	"gorm.io/gorm"
)

type Room struct {
	gorm.Model
	ScheduleID uint     `gorm:"not null;uniqueIndex"`
	Schedule   Schedule `gorm:"foreignKey:ScheduleID;references:ID;constraint:OnDelete:CASCADE"`

	Status          RoomStatus `gorm:"type:text;not null;default:SCHEDULED;index"`
	StartedAt       *time.Time `gorm:"index"`
	EndedAt         *time.Time `gorm:"index"`
	HostAbsentSince *time.Time `gorm:"index"`

	Participants []RoomParticipant `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`
}

type RoomStatus string

const (
	RoomStatusScheduled RoomStatus = "SCHEDULED"
	RoomStatusLive      RoomStatus = "LIVE"
	RoomStatusEnded     RoomStatus = "ENDED"
)
