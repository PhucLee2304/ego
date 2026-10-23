package service

import (
	"context"
	"net/http"

	"ego/platform/httpx"
	"ego/services/storage/internal/dto"
	"ego/services/storage/minio"
)

type Service interface {
	GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, *httpx.HTTPError)
}

type service struct {
	client *minio.Client
}

func New(client *minio.Client) Service {
	return &service{client: client}
}

func (s *service) GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, *httpx.HTTPError) {
	folders := []minio.BucketFolder{req.Folder}

	url, err := s.client.GeneratePresignedUploadURL(ctx, folders, req.FileName, req.ContentType)
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to generate upload URL", err)
	}

	return &dto.PresignUploadResponse{
		URL: *url,
	}, nil
}
