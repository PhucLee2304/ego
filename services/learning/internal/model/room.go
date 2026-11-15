package model

import (
	"time"

	"gorm.io/gorm"
)

type Room struct {
	gorm.Model
	ScheduleID uint     `gorm:"not null;uniqueIndex"`
	Schedule   Schedule `gorm:"foreignKey:ScheduleID;references:ID;constraint:OnDelete:CASCADE"`

	Status    RoomStatus `gorm:"type:text;not null;default:SCHEDULED;index"`
	StartedAt *time.Time `gorm:"index"`
	EndedAt   *time.Time `gorm:"index"`
	// Thời điểm host rời phòng LIVE (theo webhook LiveKit hoặc worker phát hiện), nil khi
	// host đang có mặt. Đã quá giờ kết thúc lịch mà host vắng liên tục 10 phút thì phòng tự ENDED.
	HostAbsentSince *time.Time `gorm:"index"`

	Participants []RoomParticipant `gorm:"foreignKey:RoomID;references:ID;constraint:OnDelete:CASCADE"`
}

type RoomStatus string

const (
	RoomStatusScheduled RoomStatus = "SCHEDULED"
	RoomStatusLive      RoomStatus = "LIVE"
	RoomStatusEnded     RoomStatus = "ENDED"
)
