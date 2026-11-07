package rpc

import (
	"context"
	"errors"

	storageClient "ego/api/gen/go/storage"
	platformRpc "ego/platform/rpc"
	"ego/services/storage/internal/service"
	"ego/services/storage/minio"

	"google.golang.org/grpc/codes"
)

type server struct {
	storageClient.UnimplementedStorageServiceServer
	service service.Service
}

func New(svc service.Service) storageClient.StorageServiceServer {
	return &server{service: svc}
}

// GeneratePresignedUploadURL cấp URL PUT để upload một file vào folder trong danh mục; tên file giữ
// nguyên như bên gọi truyền vào. Bên dùng:
//   - posts (CreateUploadIntent): URL cho client upload file đính kèm vào posts/community hoặc
//     posts/classrooms, posts tự đặt tên file;
//   - posts worker video (Internal = true): upload video đã chuyển mã và ảnh thumbnail vào cùng folder
//     với file gốc;
//   - seed knowledge (Internal = true): video bài học vào topics/lessons, phụ đề vào topics/transcripts;
//   - seed learning (Internal = true): audio và ảnh đề TOEIC vào exams/toeic/....
func (s *server) GeneratePresignedUploadURL(ctx context.Context, req *storageClient.GeneratePresignedUploadURLRequest) (*storageClient.GeneratePresignedUploadURLResponse, error) {
	url, err := s.service.PresignUpload(ctx, req.Folders, req.FileName, req.ContentType, req.Internal)
	if err != nil {
		return nil, toStatus(err)
	}
	return &storageClient.GeneratePresignedUploadURLResponse{Url: url}, nil
}

// GetPublicURL dựng URL công khai (không ký, không hết hạn) của một file để lưu vào DB. Bên dùng:
//   - identity (UpdateMe): avatar user trong avatars/users;
//   - learning (resolveClassroomAvatar): ảnh lớp học trong avatars/classrooms;
//   - posts (CompleteUpload) và worker video: URL file đính kèm, video đã xử lý và thumbnail;
//   - seed knowledge và learning: URL video, phụ đề và media đề thi sau khi upload.
func (s *server) GetPublicURL(ctx context.Context, req *storageClient.GetPublicURLRequest) (*storageClient.GetPublicURLResponse, error) {
	url, err := s.service.GetPublicURL(req.Folders, req.FileName)
	if err != nil {
		return nil, toStatus(err)
	}
	return &storageClient.GetPublicURLResponse{Url: url}, nil
}

// DeleteFolder xoá mọi file có prefix folder và trả số file đã xoá. Bên dùng: seed knowledge
// (cleanupTopics, xoá "topics") và seed learning (cleanupExams, xoá "exams") để dọn file đã upload
// khi lần seed bị lỗi giữa chừng.
func (s *server) DeleteFolder(ctx context.Context, req *storageClient.DeleteFolderRequest) (*storageClient.DeleteFolderResponse, error) {
	deletedCount, err := s.service.DeleteFolder(ctx, req.Folder)
	if err != nil {
		return nil, toStatus(err)
	}
	return &storageClient.DeleteFolderResponse{DeletedCount: deletedCount}, nil
}

// GetObjectMetadata trả kích thước và content type thực tế của file. Bên dùng: posts (CompleteUpload)
// để so với kích thước và loại file client khai báo lúc tạo upload, chặn file sai hoặc quá giới hạn.
func (s *server) GetObjectMetadata(ctx context.Context, req *storageClient.GetObjectMetadataRequest) (*storageClient.GetObjectMetadataResponse, error) {
	size, contentType, err := s.service.GetObjectMetadata(ctx, req.Folders, req.FileName)
	if err != nil {
		return nil, toStatus(err)
	}
	return &storageClient.GetObjectMetadataResponse{Size: size, ContentType: contentType}, nil
}

// DeleteObject xoá một file. Bên dùng posts:
//   - CancelUpload: client huỷ upload đang chờ;
//   - CleanupExpiredUploads: dọn upload hết hạn và file đính kèm không gắn với bài viết nào;
//   - worker video: xoá file gốc sau khi đã chuyển mã xong.
func (s *server) DeleteObject(ctx context.Context, req *storageClient.DeleteObjectRequest) (*storageClient.DeleteObjectResponse, error) {
	if err := s.service.DeleteObject(ctx, req.Folders, req.FileName); err != nil {
		return nil, toStatus(err)
	}
	return &storageClient.DeleteObjectResponse{}, nil
}

// toStatus map sentinel của minio sang gRPC code; lỗi khác trả nguyên để ErrorInterceptor log
// và che thành Internal.
func toStatus(err error) error {
	switch {
	case errors.Is(err, minio.ErrInvalidKey):
		return platformRpc.PublicError(codes.InvalidArgument, "[ERROR] Folder and file name are required")
	case errors.Is(err, minio.ErrInvalidFolder):
		return platformRpc.PublicError(codes.InvalidArgument, "[ERROR] Folder is not in the catalog")
	case errors.Is(err, minio.ErrObjectNotFound):
		return platformRpc.PublicError(codes.NotFound, "[NOT_FOUND] Object not found")
	}
	return err
}
