package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Notification struct {
	gorm.Model
	UserID         string            `gorm:"type:text;not null;index"`
	Title          string            `gorm:"type:text;not null"`
	Body           string            `gorm:"type:text;not null"`
	SourceService  string            `gorm:"type:text;not null;index"`
	Data           datatypes.JSONMap `gorm:"type:jsonb;not null;default:'{}'"`
	IdempotencyKey string            `gorm:"type:text;not null;uniqueIndex"`
	ReadAt         *time.Time        `gorm:"index"`
}

const (
	CategoryPosts       = "POSTS"
	CategoryClassrooms  = "CLASSROOMS"
	CategoryAssignments = "ASSIGNMENTS"
	CategoryRooms       = "ROOMS"
	CategoryExams       = "EXAMS"
)

var CategoryTypes = map[string][]string{
	CategoryPosts: {
		"CLASSROOM_POST", "CLASSROOM_POST_PINNED", "POST_LIKED",
		"POST_COMMENTED", "COMMENT_REPLIED", "COMMENT_LIKED",
	},
	CategoryClassrooms:  {"CLASSROOM"},
	CategoryAssignments: {"ASSIGNMENT"},
	CategoryRooms:       {"ROOM"},
	CategoryExams:       {"ATTEMPT"},
}

func CategoryOf(dataType string) string {
	for category, types := range CategoryTypes {
		for _, t := range types {
			if t == dataType {
				return category
			}
		}
	}
	return ""
}
