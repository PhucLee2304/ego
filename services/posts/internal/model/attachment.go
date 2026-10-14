package model

import (
	"time"

	"gorm.io/gorm"
)

type AttachmentType string
type UploadStatus string
type ProcessingStatus string

const (
	AttachmentTypeImage AttachmentType = "IMAGE"
	AttachmentTypeFile  AttachmentType = "FILE"
	AttachmentTypeVideo AttachmentType = "VIDEO"

	ProcessingStatusReady      ProcessingStatus = "READY"
	ProcessingStatusProcessing ProcessingStatus = "PROCESSING"
	ProcessingStatusFailed     ProcessingStatus = "FAILED"

	UploadStatusPending   UploadStatus = "PENDING"
	UploadStatusCompleted UploadStatus = "COMPLETED"
	UploadStatusCancelled UploadStatus = "CANCELLED"
)

type PostUploadIntent struct {
	gorm.Model
	UserID       string         `gorm:"type:text;not null;index"`
	Folder       string         `gorm:"type:text;not null"`
	FileName     string         `gorm:"type:text;not null"`
	OriginalName string         `gorm:"type:text;not null"`
	ContentType  string         `gorm:"type:text;not null"`
	DeclaredSize int64          `gorm:"not null"`
	MediaType    AttachmentType `gorm:"type:text;not null"`
	Status       UploadStatus   `gorm:"type:text;not null;index"`
	ExpiresAt    time.Time      `gorm:"not null;index"`
	AttachmentID *uint
}

type PostAttachment struct {
	gorm.Model
	PostID             *uint          `gorm:"index"`
	UserID             string         `gorm:"type:text;not null;index"`
	URL                string         `gorm:"type:text;not null"`
	ObjectKey          string         `gorm:"type:text;not null;uniqueIndex"`
	MediaType          AttachmentType `gorm:"type:text;not null"`
	OriginalName       string         `gorm:"type:text;not null"`
	ContentType        string         `gorm:"type:text;not null"`
	Size               int64          `gorm:"not null"`
	SortOrder          int            `gorm:"not null;default:0"`
	Width              *int
	Height             *int
	DurationSeconds    *float64
	ThumbnailURL       *string          `gorm:"type:text"`
	ProcessingStatus   ProcessingStatus `gorm:"type:text;not null;default:READY;index"`
	ProcessingAttempts int              `gorm:"not null;default:0"`
}
