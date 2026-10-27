package model

import "gorm.io/gorm"

// Word is the root dictionary entry for a single English word.
type Word struct {
	gorm.Model
	Word       string      `gorm:"uniqueIndex;not null"`
	Phonetic   string      `gorm:"default:''"`
	LicenseName string     `gorm:"default:''"`
	LicenseURL  string     `gorm:"default:''"`
	SourceURLs  string     `gorm:"type:text;default:''"` // JSON-encoded []string

	Phonetics   []Phonetic   `gorm:"foreignKey:WordID;constraint:OnDelete:CASCADE"`
	Meanings    []Meaning    `gorm:"foreignKey:WordID;constraint:OnDelete:CASCADE"`
}

// Phonetic holds one pronunciation variant.
type Phonetic struct {
	gorm.Model
	WordID     uint   `gorm:"not null;index"`
	Text       string `gorm:"default:''"`
	AudioPath  string `gorm:"default:''"` // relative path e.g. "assets/audios/hello-us.mp3"
	SourceURL  string `gorm:"default:''"`
	LicenseName string `gorm:"default:''"`
	LicenseURL  string `gorm:"default:''"`
}

// Meaning represents one part-of-speech group (e.g. "noun", "verb").
type Meaning struct {
	gorm.Model
	WordID      uint   `gorm:"not null;index"`
	PartOfSpeech string `gorm:"not null"`

	Definitions []Definition `gorm:"foreignKey:MeaningID;constraint:OnDelete:CASCADE"`
	Synonyms    []MeaningSynonym `gorm:"foreignKey:MeaningID;constraint:OnDelete:CASCADE"`
	Antonyms    []MeaningAntonym `gorm:"foreignKey:MeaningID;constraint:OnDelete:CASCADE"`
}

// Definition holds a single definition text + optional example.
type Definition struct {
	gorm.Model
	MeaningID  uint   `gorm:"not null;index"`
	Definition string `gorm:"type:text;not null"`
	Example    string `gorm:"type:text;default:''"`

	Synonyms []DefinitionSynonym `gorm:"foreignKey:DefinitionID;constraint:OnDelete:CASCADE"`
	Antonyms []DefinitionAntonym `gorm:"foreignKey:DefinitionID;constraint:OnDelete:CASCADE"`
}

// MeaningSynonym is a synonym at the meaning level.
type MeaningSynonym struct {
	gorm.Model
	MeaningID uint   `gorm:"not null;index"`
	Word      string `gorm:"not null"`
}

// MeaningAntonym is an antonym at the meaning level.
type MeaningAntonym struct {
	gorm.Model
	MeaningID uint   `gorm:"not null;index"`
	Word      string `gorm:"not null"`
}

// DefinitionSynonym is a synonym at the definition level.
type DefinitionSynonym struct {
	gorm.Model
	DefinitionID uint   `gorm:"not null;index"`
	Word         string `gorm:"not null"`
}

// DefinitionAntonym is an antonym at the definition level.
type DefinitionAntonym struct {
	gorm.Model
	DefinitionID uint   `gorm:"not null;index"`
	Word         string `gorm:"not null"`
}
