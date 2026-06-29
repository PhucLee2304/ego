package model

import "gorm.io/gorm"

type Question struct {
	gorm.Model
	Content     string    `gorm:"type:text;not null"`
	Part        *PartCode `gorm:"type:text;index"`
	Explanation string    `gorm:"type:text"`
	Order       int       `gorm:"not null"`

	SectionID uint    `gorm:"not null;index"`
	Section   Section `gorm:"foreignKey:SectionID;references:ID"`
	GroupID   *uint   `gorm:"index"`
	Group     *Group  `gorm:"foreignKey:GroupID;references:ID"`

	Options []Option `gorm:"foreignKey:QuestionID;references:ID"`
}

type PartCode string

const (
	Part1 PartCode = "1"
	Part2 PartCode = "2"
	Part3 PartCode = "3"
	Part4 PartCode = "4"
	Part5 PartCode = "5"
	Part6 PartCode = "6"
	Part7 PartCode = "7"
)
