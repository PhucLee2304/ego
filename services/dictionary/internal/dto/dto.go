package dto

// ---- Phonetic DTO ----

type PhoneticResponse struct {
	Text      string `json:"text"`
	AudioURL  string `json:"audioUrl,omitempty"` // full URL built from base URL + normalized path
	SourceURL string `json:"sourceUrl,omitempty"`
}

// ---- Definition DTO ----

type DefinitionResponse struct {
	Definition string   `json:"definition"`
	Example    string   `json:"example,omitempty"`
	Synonyms   []string `json:"synonyms"`
	Antonyms   []string `json:"antonyms"`
}

// ---- Meaning DTO ----

type MeaningResponse struct {
	PartOfSpeech string               `json:"partOfSpeech"`
	Definitions  []DefinitionResponse `json:"definitions"`
	Synonyms     []string             `json:"synonyms"`
	Antonyms     []string             `json:"antonyms"`
}

// ---- Word DTO (full lookup) ----

type WordResponse struct {
	ID          uint               `json:"id"`
	Word        string             `json:"word"`
	Phonetic    string             `json:"phonetic,omitempty"`
	Phonetics   []PhoneticResponse `json:"phonetics"`
	Meanings    []MeaningResponse  `json:"meanings"`
	LicenseName string             `json:"licenseName,omitempty"`
	LicenseURL  string             `json:"licenseUrl,omitempty"`
	SourceURLs  []string           `json:"sourceUrls,omitempty"`
}

// ---- Word Summary DTO (for search results) ----

type WordSummaryResponse struct {
	ID       uint   `json:"id"`
	Word     string `json:"word"`
	Phonetic string `json:"phonetic,omitempty"`
}

// ---- Search DTO ----

type SearchQuery struct {
	Q        string `schema:"q"`
	Page     int    `schema:"page"`
	PageSize int    `schema:"pageSize"`
}

func (q *SearchQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 10
	} else if q.PageSize > 50 {
		q.PageSize = 50
	}
}

type SearchResponse struct {
	Data       []WordSummaryResponse `json:"data"`
	Page       int                   `json:"page"`
	PageCounts int                   `json:"pageCounts"`
	Total      int64                 `json:"total"`
}

// ---- Thesaurus DTO ----

type ThesaurusResponse struct {
	Word     string   `json:"word"`
	Synonyms []string `json:"synonyms"`
	Antonyms []string `json:"antonyms"`
}

// ---- Flashcard DTOs ----

type AddFlashcardRequest struct {
	WordID uint   `json:"wordId" validate:"required"`
	Note   string `json:"note"`
}

type FlashcardResponse struct {
	ID        uint                `json:"id"`
	Note      string              `json:"note"`
	CreatedAt string              `json:"createdAt"`
	Word      WordSummaryResponse `json:"word"`
}

type GetFlashcardsQuery struct {
	Page     int `schema:"page"`
	PageSize int `schema:"pageSize"`
}

func (q *GetFlashcardsQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 10
	} else if q.PageSize > 100 {
		q.PageSize = 100
	}
}

type GetFlashcardsResponse struct {
	Data       []FlashcardResponse `json:"data"`
	Page       int                 `json:"page"`
	PageCounts int                 `json:"pageCounts"`
	Total      int64               `json:"total"`
}
