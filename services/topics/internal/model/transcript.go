package model

import "gorm.io/gorm"

type Transcript struct {
	gorm.Model
	Content   string  `gorm:"not null"`
	Order     uint    `gorm:"not null"`
	TimeStart float64 `gorm:"not null"`
	TimeEnd   float64 `gorm:"not null"`
	Url       string
	LessonID  uint   `gorm:"not null"`
	Lesson    Lesson `gorm:"foreignKey:LessonID;references:ID"`
}
