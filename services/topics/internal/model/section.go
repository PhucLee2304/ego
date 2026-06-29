package model

import "gorm.io/gorm"

type Section struct {
	gorm.Model
	Name    string `gorm:"not null"`
	TopicID uint   `gorm:"not null;index"`
	Topic   Topic  `gorm:"foreignKey:TopicID;references:ID"`
}
