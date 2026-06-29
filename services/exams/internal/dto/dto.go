package dto

import "time"

type Exam struct {
	ID          uint      `json:"id"`
	Title       string    `json:"title"`
	Description *string   `json:"description,omitempty"`
	IsPublic    bool      `json:"isPublic"`
	Type        string    `json:"type"`
	Year        *int      `json:"year,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
