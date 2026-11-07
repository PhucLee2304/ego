package service

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"ego/platform/httpx"
	"ego/platform/storagekey"
	"ego/services/storage/internal/dto"
	"ego/services/storage/minio"
)

// GeneratePresignUploadURL cấp URL PUT cho client, dùng chung cho mọi folder trong danh mục:
//   - folder là đường dẫn đúng 2 cấp như "avatars/users" hay "posts/community"; sai số cấp hoặc không
//     nằm trong danh mục thì trả 400 (cấp sâu hơn chỉ service sở hữu đặt qua rpc);
//   - tên file do server đặt bằng storagekey.FileName từ tên gốc (chuẩn hoá, thêm timestamp và đoạn
//     ngẫu nhiên), nên client không xin được URL ghi đè file đã có;
//   - trả về url để PUT và fileName để client gửi cho service sở hữu dữ liệu khi gắn file.
func (s *service) GeneratePresignUploadURL(ctx context.Context, req dto.PresignUploadRequest) (*dto.PresignUploadResponse, *httpx.HTTPError) {
	folders := strings.Split(req.Folder, "/")
	if len(folders) != 2 {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Folder is not allowed", minio.ErrInvalidFolder)
	}
	fileName := storagekey.FileName(req.FileName)

	url, err := s.PresignUpload(ctx, folders, fileName, req.ContentType, false)
	if errors.Is(err, minio.ErrInvalidFolder) {
		return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Folder is not allowed", err)
	}
	if err != nil {
		return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to generate upload URL", err)
	}
	return &dto.PresignUploadResponse{URL: url, FileName: fileName}, nil
}

// PresignUpload cấp URL PUT cho folders/fileName. folders phải nằm trong danh mục của minio
// (ErrInvalidFolder nếu không); tên file giữ nguyên như bên gọi truyền vào. internal = true ký URL
// theo địa chỉ MinIO trong mạng docker, dùng khi service tự upload (seed, worker video) thay vì client.
func (s *service) PresignUpload(ctx context.Context, folders []string, fileName, contentType string, internal bool) (string, error) {
	url, err := s.store.GeneratePresignedUploadURL(ctx, toBucketFolders(folders), fileName, contentType, internal)
	if err != nil {
		return "", err
	}
	return *url, nil
}
