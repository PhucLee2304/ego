package dto

import (
	"ego/platform/httpx"
	"ego/services/knowledge/internal/model"
)

type Theme struct {
	ID          uint    `json:"id"`
	Category    string  `json:"category"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Icon        *string `json:"icon"`
	Level       *string `json:"level"`
	WordCount   int     `json:"wordCount"`
}

// ThemeWord là từ trong theme. ID = 0 khi từ chưa có trong từ điển.
type ThemeWord struct {
	ID       uint    `json:"id"`
	Word     string  `json:"word"`
	Phonetic *string `json:"phonetic,omitempty"`
}

// GetThemeWordsResponse là trang từ của theme, kèm thông tin theme.
type GetThemeWordsResponse struct {
	httpx.PaginatedResponse[*ThemeWord]
	Topic *Theme `json:"topic"`
}

func ToTheme(theme *model.Theme) *Theme {
	return &Theme{
		ID:          theme.ID,
		Category:    theme.Category,
		Name:        theme.Name,
		Description: theme.Description,
		Icon:        theme.Icon,
		Level:       theme.Level,
		WordCount:   theme.WordCount,
	}
}

func ToThemeWord(word *model.Word) *ThemeWord {
	return &ThemeWord{
		ID:       word.ID,
		Word:     word.Word,
		Phonetic: word.Phonetic,
	}
}
