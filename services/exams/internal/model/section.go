package model

import "gorm.io/gorm"

type Section struct {
	gorm.Model
	Code     SectionCode `gorm:"type:text;not null;index"`
	Title    string      `gorm:"not null"`
	Order    int         `gorm:"not null"`
	Duration *int

	ExamID uint `gorm:"not null;index"`
	Exam   Exam `gorm:"foreignKey:ExamID;references:ID"`

	Groups    []Group    `gorm:"foreignKey:SectionID;references:ID"`
	Questions []Question `gorm:"foreignKey:SectionID;references:ID"`
}

type SectionCode string

const (
	SectionCodeFull      SectionCode = "FULL"
	SectionCodeListening SectionCode = "LISTENING"
	SectionCodeReading   SectionCode = "READING"
)
