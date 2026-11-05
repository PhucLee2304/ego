package dto

import (
	"ego/platform/httpx"
	"ego/services/knowledge/internal/model"
)

type GetWordsQuery struct {
	httpx.PaginationQuery
	Query string `schema:"query"`
}

type Phonetic struct {
	Text      *string `json:"text"`
	AudioURL  *string `json:"audioUrl"`
	SourceURL *string `json:"sourceUrl"`
}

type Definition struct {
	Definition string   `json:"definition"`
	Example    *string  `json:"example"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
}

type Meaning struct {
	PartOfSpeech string        `json:"partOfSpeech"`
	Definitions  []*Definition `json:"definitions"`
	Synonyms     []string      `json:"synonyms"`
	Antonyms     []string      `json:"antonyms"`
}

// Word dùng chung cho danh sách và chi tiết. Danh sách không preload nên
// phonetics/meanings là mảng rỗng; chi tiết có đủ.
type Word struct {
	ID         uint        `json:"id"`
	Word       string      `json:"word"`
	Phonetic   *string     `json:"phonetic"`
	Phonetics  []*Phonetic `json:"phonetics"`
	Meanings   []*Meaning  `json:"meanings"`
	SourceURLs []string    `json:"sourceUrls"`
}

func ToWord(word *model.Word) *Word {
	phonetics := make([]*Phonetic, len(word.Phonetics))
	for i := range word.Phonetics {
		phonetics[i] = ToPhonetic(&word.Phonetics[i])
	}

	meanings := make([]*Meaning, len(word.Meanings))
	for i := range word.Meanings {
		meanings[i] = ToMeaning(&word.Meanings[i])
	}

	return &Word{
		ID:         word.ID,
		Word:       word.Word,
		Phonetic:   word.Phonetic,
		Phonetics:  phonetics,
		Meanings:   meanings,
		SourceURLs: word.SourceUrls,
	}
}

func ToPhonetic(phonetic *model.Phonetic) *Phonetic {
	return &Phonetic{
		Text:      phonetic.Text,
		AudioURL:  phonetic.AudioUrl,
		SourceURL: phonetic.SourceUrl,
	}
}

func ToMeaning(meaning *model.Meaning) *Meaning {
	definitions := make([]*Definition, len(meaning.Definitions))
	for i := range meaning.Definitions {
		definitions[i] = ToDefinition(&meaning.Definitions[i])
	}

	return &Meaning{
		PartOfSpeech: string(meaning.PartOfSpeech),
		Definitions:  definitions,
		Synonyms:     meaning.Synonyms,
		Antonyms:     meaning.Antonyms,
	}
}

func ToDefinition(definition *model.Definition) *Definition {
	return &Definition{
		Definition: definition.DefinitionText,
		Example:    definition.Example,
		Synonyms:   definition.Synonyms,
		Antonyms:   definition.Antonyms,
	}
}
