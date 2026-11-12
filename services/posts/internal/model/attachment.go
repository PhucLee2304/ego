package model

import (
	"time"

	"gorm.io/gorm"
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

	AttachmentID *uint           `gorm:"index"`
	Attachment   *PostAttachment `gorm:"foreignKey:AttachmentID;references:ID;constraint:OnDelete:SET NULL"`
}

type PostAttachment struct {
	gorm.Model
	PostID *uint  `gorm:"index"`
	Post   *Post  `gorm:"foreignKey:PostID;references:ID"`
	UserID string `gorm:"type:text;not null;index"`

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

type UploadStatus string

const (
	UploadStatusPending   UploadStatus = "PENDING"
	UploadStatusCompleted UploadStatus = "COMPLETED"
	UploadStatusCancelled UploadStatus = "CANCELLED"
)

type AttachmentType string

const (
	AttachmentTypeImage AttachmentType = "IMAGE"
	AttachmentTypeFile  AttachmentType = "FILE"
	AttachmentTypeVideo AttachmentType = "VIDEO"
)

type ProcessingStatus string

const (
	ProcessingStatusReady      ProcessingStatus = "READY"
	ProcessingStatusProcessing ProcessingStatus = "PROCESSING"
	ProcessingStatusFailed     ProcessingStatus = "FAILED"
)
