package dto

type PresignUploadRequest struct {
	// FileName là tên gốc của file; server chuẩn hoá và thêm timestamp, client không chọn key.
	FileName    string `json:"fileName" validate:"required,notblank"`
	ContentType string `json:"contentType" validate:"required,notblank"`
	// Folder là đường dẫn trong danh mục, ví dụ "avatars/users", "posts/community", "exams/toeic/audio".
	Folder string `json:"folder" validate:"required"`
}

type PresignUploadResponse struct {
	URL string `json:"url"`
	// FileName là tên file server đã đặt; client gửi lại cho service sở hữu dữ liệu khi gắn file.
	FileName string `json:"fileName"`
}
