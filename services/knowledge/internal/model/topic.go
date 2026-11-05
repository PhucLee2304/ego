package model

import "gorm.io/gorm"

type Topic struct {
	gorm.Model
	Name    string `gorm:"not null"`
	IsAudio bool   `gorm:"default:false"`
	Url     string `gorm:"not null"`
}

type Section struct {
	gorm.Model
	Name    string `gorm:"not null"`
	TopicID uint   `gorm:"not null;index"`
	Topic   Topic  `gorm:"foreignKey:TopicID;references:ID"`
}

type Lesson struct {
	gorm.Model
	Title       string       `gorm:"not null"`
	Description string       `gorm:"not null"`
	Subtitle    string       `gorm:"not null"`
	Url         string       `gorm:"not null"`
	SectionID   uint         `gorm:"not null;index"`
	Section     Section      `gorm:"foreignKey:SectionID;references:ID"`
	Transcripts []Transcript `gorm:"foreignKey:LessonID"`
}

type Transcript struct {
	gorm.Model
	Content   string  `gorm:"not null"`
	Order     uint    `gorm:"not null"`
	TimeStart float64 `gorm:"not null"`
	TimeEnd   float64 `gorm:"not null"`
	Url       string
	LessonID  uint   `gorm:"not null;index"`
	Lesson    Lesson `gorm:"foreignKey:LessonID;references:ID"`
}
