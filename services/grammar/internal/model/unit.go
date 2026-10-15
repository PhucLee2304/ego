package model

import "gorm.io/gorm"

// GrammarUnit is a single grammar learning unit (e.g. "Unit 1: Pronouns").
type GrammarUnit struct {
	gorm.Model
	Unit     int    `gorm:"uniqueIndex;not null"`
	Title    string `gorm:"not null"`
	Theory   string `gorm:"type:text;not null"`

	Examples  []GrammarExample  `gorm:"foreignKey:UnitID;constraint:OnDelete:CASCADE"`
	Practices []GrammarPractice `gorm:"foreignKey:UnitID;constraint:OnDelete:CASCADE"`
}

// GrammarExample holds one example sentence for a unit.
type GrammarExample struct {
	gorm.Model
	UnitID  uint   `gorm:"not null;index"`
	Content string `gorm:"type:text;not null"`
}

// GrammarPractice holds one MCQ practice question for a unit.
type GrammarPractice struct {
	gorm.Model
	UnitID      uint   `gorm:"not null;index"`
	Question    string `gorm:"type:text;not null"`
	Options     string `gorm:"type:text;not null"`  // JSON-encoded []string
	Correct     string `gorm:"not null"`
	Explanation string `gorm:"type:text;not null"`
	SortOrder   int    `gorm:"default:0"`
}

// UserUnitProgress tracks whether a user has completed a unit's practice.
type UserUnitProgress struct {
	gorm.Model
	UserID    string `gorm:"not null;index"`
	UnitID    uint   `gorm:"not null;index"`
	Completed bool   `gorm:"default:false"`
}
