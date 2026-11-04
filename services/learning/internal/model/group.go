package model

import "gorm.io/gorm"

type Group struct {
	gorm.Model
	Title       string
	Instruction string    `gorm:"type:text"`
	AudioURL    *string   `gorm:"type:text"`
	ImageURLs   []string  `gorm:"serializer:json;type:jsonb"`
	Transcript  string    `gorm:"type:text"`
	Explanation string    `gorm:"type:text"`
	Order       int       `gorm:"not null"`
	Part        *PartCode `gorm:"type:text"`

	// Đo lúc seed bằng ffprobe/silencedetect, dùng để tính gapMs của trang nghe.
	AudioDurationMs   *int
	TrailingSilenceMs *int

	SectionID uint    `gorm:"not null;index"`
	Section   Section `gorm:"foreignKey:SectionID;references:ID"`

	Questions []Question `gorm:"foreignKey:GroupID;references:ID"`
}
