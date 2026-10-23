package model

import "gorm.io/gorm"

type Group struct {
	gorm.Model
	Title       string
	Instruction string  `gorm:"type:text"`
	AudioURL    *string `gorm:"type:text"`
	ImageURL    *string `gorm:"type:text"`
	Transcript  string  `gorm:"type:text"`
	Explanation string  `gorm:"type:text"`
	Order       int     `gorm:"not null"`

	SectionID uint    `gorm:"not null;index"`
	Section   Section `gorm:"foreignKey:SectionID;references:ID"`

	Questions []Question `gorm:"foreignKey:GroupID;references:ID"`
}
