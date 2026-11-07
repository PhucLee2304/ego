package service

import (
	"context"

	"ego/platform/httpx"
	"ego/services/storage/internal/dto"
	"ego/services/storage/minio"
)

// Service gom mọi thao tác với bucket. Handler HTTP và gRPC server đều gọi qua đây.
type Service interface {
	// Upload
	// GeneratePresignUploadURL phục vụ HTTP: client upload vào folder bất kỳ trong danh mục, server đặt tên file.
	GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, *httpx.HTTPError)
	// PresignUpload là hàm cấp URL PUT chung cho mọi folder trong danh mục; rpc và HTTP đều dùng.
	PresignUpload(ctx context.Context, folders []string, fileName, contentType string, internal bool) (string, error)

	// Object (gọi nội bộ qua rpc)
	GetPublicURL(folders []string, fileName string) (string, error)
	GetObjectMetadata(ctx context.Context, folders []string, fileName string) (int64, string, error)
	DeleteObject(ctx context.Context, folders []string, fileName string) error
	DeleteFolder(ctx context.Context, folder string) (uint32, error)
}

// ObjectStore là phần của minio.Client mà service dùng, khai báo hẹp để test bằng fake.
type ObjectStore interface {
	GeneratePresignedUploadURL(ctx context.Context, folders []minio.BucketFolder, fileName, contentType string, internal bool) (*string, error)
	GetPublicURL(folders []minio.BucketFolder, fileName string) (*string, error)
	GetObjectMetadata(ctx context.Context, folders []minio.BucketFolder, fileName string) (int64, string, error)
	DeleteObject(ctx context.Context, folders []minio.BucketFolder, fileName string) error
	DeleteFolder(ctx context.Context, folder string) (uint32, error)
}

type service struct {
	store ObjectStore
}

func New(store ObjectStore) Service {
	return &service{store: store}
}

func toBucketFolders(folders []string) []minio.BucketFolder {
	result := make([]minio.BucketFolder, len(folders))
	for i, f := range folders {
		result[i] = minio.BucketFolder(f)
	}
	return result
}
