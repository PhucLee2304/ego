package model

import "gorm.io/gorm"

type Lesson struct {
	gorm.Model
	Title       string  `gorm:"not null"`
	Description string  `gorm:"not null"`
	Subtitle    string  `gorm:"not null"`
	Url         string  `gorm:"not null"`
	SectionID   uint         `gorm:"not null"`
	Section     Section      `gorm:"foreignKey:SectionID;references:ID"`
	Transcripts []Transcript `gorm:"foreignKey:LessonID"`
}
