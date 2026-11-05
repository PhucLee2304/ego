package model

import "gorm.io/gorm"

// Theme là chủ đề từ vựng (Business, Travel…).
type Theme struct {
	gorm.Model
	Category    string  `gorm:"type:text;not null"`
	Name        string  `gorm:"type:text;not null"`
	Description *string `gorm:"type:text"`
	Icon        *string `gorm:"type:text"`
	Level       *string `gorm:"type:text"`
	OrderIndex  int     `gorm:"not null;default:0"`
	WordCount   int     `gorm:"not null;default:0"`

	Words []ThemeWord `gorm:"foreignKey:ThemeID;references:ID"`
}

// ThemeWord là một từ (dạng chữ) thuộc theme. Khi đọc, từ được khớp sang bảng
// words theo chữ thường; từ không có trong từ điển vẫn được giữ.
type ThemeWord struct {
	gorm.Model
	Word     string `gorm:"type:text;not null;uniqueIndex:idx_theme_words_theme_word"`
	OrderNum int    `gorm:"not null;default:0"`

	ThemeID uint  `gorm:"not null;index;uniqueIndex:idx_theme_words_theme_word"`
	Theme   Theme `gorm:"foreignKey:ThemeID;references:ID;constraint:OnDelete:CASCADE"`
}
