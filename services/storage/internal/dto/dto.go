package dto

import "ego/services/storage/minio"

type PresignUploadRequest struct {
	FileName    string             `json:"fileName" validate:"required,notblank"`
	ContentType string             `json:"contentType" validate:"required,notblank"`
	Folder      minio.BucketFolder `json:"folder" validate:"required"`
}

type PresignUploadResponse struct {
	URL string `json:"url"`
}
