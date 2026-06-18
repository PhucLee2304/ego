package rpc

import (
	"context"
	"fmt"
	"strings"

	storageClient "ego/api/gen/go/storage"
	"ego/services/storage/minio"
)

type server struct {
	storageClient.UnimplementedStorageServiceServer
	minioClient *minio.Client
}

func New(minioClient *minio.Client) storageClient.StorageServiceServer {
	return &server{
		minioClient: minioClient,
	}
}

func (s *server) GeneratePresignedUploadURL(ctx context.Context, req *storageClient.GeneratePresignedUploadURLRequest) (*storageClient.GeneratePresignedUploadURLResponse, error) {
	folders := toBucketFolders(req.Folders)
	url, err := s.minioClient.GeneratePresignedUploadURL(ctx, folders, req.FileName, req.ContentType)
	if err != nil {
		return nil, err
	}
	return &storageClient.GeneratePresignedUploadURLResponse{
		Url: *url,
	}, nil
}

func (s *server) GeneratePresignedDownloadURL(ctx context.Context, req *storageClient.GeneratePresignedDownloadURLRequest) (*storageClient.GeneratePresignedDownloadURLResponse, error) {
	folders := toBucketFolders(req.Folders)
	url, err := s.minioClient.GeneratePresignedDownloadURL(ctx, folders, req.FileName)
	if err != nil {
		return nil, err
	}
	return &storageClient.GeneratePresignedDownloadURLResponse{
		Url: *url,
	}, nil
}

func (s *server) GetPublicURL(ctx context.Context, req *storageClient.GetPublicURLRequest) (*storageClient.GetPublicURLResponse, error) {
	folders := toBucketFolders(req.Folders)
	objectKey := minio.BuildObjectKey(folders, req.FileName)
	url := fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.minioClient.Endpoint, "/"), s.minioClient.Bucket, objectKey)
	return &storageClient.GetPublicURLResponse{
		Url: url,
	}, nil
}

func toBucketFolders(folders []string) []minio.BucketFolder {
	result := make([]minio.BucketFolder, len(folders))
	for i, f := range folders {
		result[i] = minio.BucketFolder(f)
	}
	return result
}
