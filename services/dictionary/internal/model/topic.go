package model

import "time"

// Topic represents a vocabulary theme/topic.
type Topic struct {
	ID          string      `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Category    string      `gorm:"type:varchar(64);not null;index" json:"category"`
	Name        string      `gorm:"type:varchar(255);not null" json:"name"`
	Description string      `gorm:"type:text;default:''" json:"description"`
	Icon        string      `gorm:"type:varchar(64);default:''" json:"icon"`
	Level       string      `gorm:"type:varchar(64);default:''" json:"level"`
	OrderIndex  int         `gorm:"default:0" json:"orderIndex"`
	WordCount   int         `gorm:"default:0" json:"wordCount"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`

	Words       []TopicWord `gorm:"foreignKey:TopicID;constraint:OnDelete:CASCADE" json:"-"`
}

// TopicWord links a topic with an English word.
type TopicWord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TopicID   string    `gorm:"type:varchar(64);not null;index:idx_topic_word,unique;index" json:"topicId"`
	Word      string    `gorm:"type:varchar(128);not null;index:idx_topic_word,unique;index" json:"word"`
	OrderNum  int       `gorm:"default:0" json:"orderNum"`
	CreatedAt time.Time `json:"createdAt"`
}
