package service

import "context"

// Các method ở file này chỉ được rpc (rpc/rpc.go) gọi cho service khác dùng nội bộ, không có route HTTP.
// Chúng trả error kèm sentinel của minio (ErrInvalidKey, ErrObjectNotFound) để rpc map sang gRPC code.

// GetPublicURL dựng URL công khai (không ký, không hết hạn) của file để lưu vào DB.
// Gọi từ rpc GetPublicURL, bên dùng:
//   - identity (UpdateMe): avatar user trong avatars/users;
//   - learning (resolveClassroomAvatar): ảnh lớp học trong avatars/classrooms;
//   - posts (CompleteUpload) và worker video: file đính kèm, video đã xử lý, thumbnail;
//   - seed knowledge và learning: video, phụ đề, media đề thi sau khi upload.
func (s *service) GetPublicURL(folders []string, fileName string) (string, error) {
	url, err := s.store.GetPublicURL(toBucketFolders(folders), fileName)
	if err != nil {
		return "", err
	}
	return *url, nil
}

// GetObjectMetadata trả kích thước và content type thực tế của file (ErrObjectNotFound nếu chưa có).
// Gọi từ rpc GetObjectMetadata, bên dùng: posts (CompleteUpload) để so với kích thước và loại file
// client khai báo lúc tạo upload, chặn file sai hoặc vượt giới hạn.
func (s *service) GetObjectMetadata(ctx context.Context, folders []string, fileName string) (int64, string, error) {
	return s.store.GetObjectMetadata(ctx, toBucketFolders(folders), fileName)
}

// DeleteObject xoá một file. Gọi từ rpc DeleteObject, bên dùng posts:
//   - CancelUpload: client huỷ upload đang chờ;
//   - CleanupExpiredUploads: dọn upload hết hạn và file đính kèm không gắn bài viết nào;
//   - worker video: xoá file gốc sau khi chuyển mã xong.
func (s *service) DeleteObject(ctx context.Context, folders []string, fileName string) error {
	return s.store.DeleteObject(ctx, toBucketFolders(folders), fileName)
}

// DeleteFolder xoá mọi file có prefix folder, trả số file đã xoá. Gọi từ rpc DeleteFolder, bên dùng:
// seed knowledge (cleanupTopics, xoá "topics") và seed learning (cleanupExams, xoá "exams") để dọn
// file đã upload khi lần seed lỗi giữa chừng.
func (s *service) DeleteFolder(ctx context.Context, folder string) (uint32, error) {
	return s.store.DeleteFolder(ctx, folder)
}
