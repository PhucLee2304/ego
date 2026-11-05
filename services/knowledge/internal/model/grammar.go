package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// GrammarUnit là một bài ngữ pháp (Unit 1: Pronouns…) gồm lý thuyết, ví dụ và
// câu hỏi luyện tập.
type GrammarUnit struct {
	gorm.Model
	Unit   int    `gorm:"not null;uniqueIndex"`
	Title  string `gorm:"type:text;not null"`
	Theory string `gorm:"type:text;not null"`

	Examples   []GrammarExample   `gorm:"foreignKey:UnitID;references:ID"`
	Practices  []GrammarPractice  `gorm:"foreignKey:UnitID;references:ID"`
	Progresses []UserUnitProgress `gorm:"foreignKey:UnitID;references:ID"`
}

// GrammarExample là một câu ví dụ của unit.
type GrammarExample struct {
	gorm.Model
	Content string `gorm:"type:text;not null"`

	UnitID uint        `gorm:"not null;index"`
	Unit   GrammarUnit `gorm:"foreignKey:UnitID;references:ID;constraint:OnDelete:CASCADE"`
}

// GrammarPractice là một câu hỏi trắc nghiệm của unit.
type GrammarPractice struct {
	gorm.Model
	Question    string                      `gorm:"type:text;not null"`
	Options     datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_grammar_practices_options_array,jsonb_typeof(options) = 'array'"`
	Correct     string                      `gorm:"type:text;not null"`
	Explanation string                      `gorm:"type:text;not null"`
	SortOrder   int                         `gorm:"not null;default:0"`

	UnitID uint        `gorm:"not null;index"`
	Unit   GrammarUnit `gorm:"foreignKey:UnitID;references:ID;constraint:OnDelete:CASCADE"`
}

// UserUnitProgress là trạng thái hoàn thành phần luyện tập của một người với
// một unit. Mỗi cặp (user, unit) chỉ có một dòng.
type UserUnitProgress struct {
	gorm.Model
	UserID    string `gorm:"type:text;not null;uniqueIndex:idx_user_unit_progresses_user_unit"`
	Completed bool   `gorm:"not null;default:false"`

	UnitID uint        `gorm:"not null;index;uniqueIndex:idx_user_unit_progresses_user_unit"`
	Unit   GrammarUnit `gorm:"foreignKey:UnitID;references:ID;constraint:OnDelete:CASCADE"`
}
