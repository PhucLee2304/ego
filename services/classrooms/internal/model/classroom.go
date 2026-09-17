package model

import "gorm.io/gorm"

type Classroom struct {
	gorm.Model
	Code        string  `gorm:"type:text;not null;uniqueIndex"`
	Name        string  `gorm:"type:text;not null"`
	Description *string `gorm:"type:text"`
	Avatar      *string `gorm:"type:text"`

	TeacherID   string `gorm:"type:text;not null;index"`
	Active      bool   `gorm:"not null;default:true;index"`
	MaxStudents int    `gorm:"not null;default:50"`

	Members     []Member     `gorm:"foreignKey:ClassroomID;references:ID"`
	Events      []Event      `gorm:"foreignKey:ClassroomID;references:ID"`
	Assignments []Assignment `gorm:"foreignKey:ClassroomID;references:ID"`
	Rooms       []Room       `gorm:"foreignKey:ClassroomID;references:ID"`
}
