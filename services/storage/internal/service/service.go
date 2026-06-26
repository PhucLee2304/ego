package service

import (
	"context"
	"ego/services/storage/internal/dto"
	"ego/services/storage/minio"
)

type Service interface {
	GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, error)
}

type service struct {
	client *minio.Client
}

func New(client *minio.Client) Service {
	return &service{client: client}
}

func (s *service) GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, error) {
	folders := []minio.BucketFolder{req.Folder}

	url, err := s.client.GeneratePresignedUploadURL(ctx, folders, req.FileName, req.ContentType)
	if err != nil {
		return nil, err
	}

	return &dto.PresignUploadResponse{
		URL: *url,
	}, nil
}
