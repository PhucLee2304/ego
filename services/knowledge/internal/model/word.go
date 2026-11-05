package model

import (
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Word là một mục từ điển. Mỗi key trong dữ liệu seed là một Word; các entry
// đồng âm của cùng một key được gộp phonetics/meanings theo thứ tự.
// Word luôn lưu chữ thường; unique index nằm ở index.go.
// Cột jsonb mảng có CHECK jsonb_typeof = 'array': JSONSlice nil được GORM ghi
// thành jsonb null (không bị NOT NULL chặn), nên chặn ở DB để luôn đọc ra mảng.
type Word struct {
	gorm.Model
	Word       string `gorm:"type:text;not null"`
	Phonetic   *string
	SourceUrls datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_words_source_urls_array,jsonb_typeof(source_urls) = 'array'"`
	Phonetics  []Phonetic                  `gorm:"foreignKey:WordID;constraint:OnDelete:CASCADE"`
	Meanings   []Meaning                   `gorm:"foreignKey:WordID;constraint:OnDelete:CASCADE"`
}

// Phonetic là một cách phát âm. AudioUrl là public URL của file trên MinIO.
// License là của file audio (mỗi file một license riêng).
type Phonetic struct {
	gorm.Model
	Text        *string
	AudioUrl    *string
	SourceUrl   *string
	LicenseName *string
	LicenseUrl  *string
	WordID      uint `gorm:"not null;index"`
	Word        Word `gorm:"foreignKey:WordID;references:ID"`
}

type PartOfSpeech string

const (
	PartOfSpeechNoun         PartOfSpeech = "noun"
	PartOfSpeechProperNoun   PartOfSpeech = "proper noun"
	PartOfSpeechVerb         PartOfSpeech = "verb"
	PartOfSpeechAdjective    PartOfSpeech = "adjective"
	PartOfSpeechAdverb       PartOfSpeech = "adverb"
	PartOfSpeechPronoun      PartOfSpeech = "pronoun"
	PartOfSpeechPreposition  PartOfSpeech = "preposition"
	PartOfSpeechConjunction  PartOfSpeech = "conjunction"
	PartOfSpeechInterjection PartOfSpeech = "interjection"
	PartOfSpeechNumeral      PartOfSpeech = "numeral"
)

// Meaning là một nhóm nghĩa theo từ loại.
type Meaning struct {
	gorm.Model
	PartOfSpeech PartOfSpeech                `gorm:"type:text;not null"`
	Synonyms     datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_meanings_synonyms_array,jsonb_typeof(synonyms) = 'array'"`
	Antonyms     datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_meanings_antonyms_array,jsonb_typeof(antonyms) = 'array'"`
	WordID       uint                        `gorm:"not null;index"`
	Word         Word                        `gorm:"foreignKey:WordID;references:ID"`
	Definitions  []Definition                `gorm:"foreignKey:MeaningID;constraint:OnDelete:CASCADE"`
}

// Definition là một định nghĩa kèm ví dụ (nếu có).
type Definition struct {
	gorm.Model
	DefinitionText string                      `gorm:"type:text;not null"`
	Example        *string                     `gorm:"type:text"`
	Synonyms       datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_definitions_synonyms_array,jsonb_typeof(synonyms) = 'array'"`
	Antonyms       datatypes.JSONSlice[string] `gorm:"type:jsonb;not null;default:'[]';check:chk_definitions_antonyms_array,jsonb_typeof(antonyms) = 'array'"`
	MeaningID      uint                        `gorm:"not null;index"`
	Meaning        Meaning                     `gorm:"foreignKey:MeaningID;references:ID"`
}
