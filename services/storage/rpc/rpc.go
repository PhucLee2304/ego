package rpc

import (
	"context"

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
	url, err := s.minioClient.GetPublicURL(folders, req.FileName)
	if err != nil {
		return nil, err
	}
	return &storageClient.GetPublicURLResponse{
		Url: *url,
	}, nil
}

func (s *server) DeleteFolder(ctx context.Context, req *storageClient.DeleteFolderRequest) (*storageClient.DeleteFolderResponse, error) {
	deletedCount, err := s.minioClient.DeleteFolder(ctx, req.Folder)
	if err != nil {
		return nil, err
	}
	return &storageClient.DeleteFolderResponse{
		DeletedCount: deletedCount,
	}, nil
}

func (s *server) GetObjectMetadata(ctx context.Context, req *storageClient.GetObjectMetadataRequest) (*storageClient.GetObjectMetadataResponse, error) {
	size, contentType, err := s.minioClient.GetObjectMetadata(ctx, toBucketFolders(req.Folders), req.FileName)
	if err != nil {
		return nil, err
	}
	return &storageClient.GetObjectMetadataResponse{Size: size, ContentType: contentType}, nil
}

func (s *server) DeleteObject(ctx context.Context, req *storageClient.DeleteObjectRequest) (*storageClient.DeleteObjectResponse, error) {
	if err := s.minioClient.DeleteObject(ctx, toBucketFolders(req.Folders), req.FileName); err != nil {
		return nil, err
	}
	return &storageClient.DeleteObjectResponse{}, nil
}

func toBucketFolders(folders []string) []minio.BucketFolder {
	result := make([]minio.BucketFolder, len(folders))
	for i, f := range folders {
		result[i] = minio.BucketFolder(f)
	}
	return result
}
