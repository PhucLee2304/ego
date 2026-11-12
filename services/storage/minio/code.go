package minio

import (
	"errors"
	"slices"
	"strings"
)

type BucketFolder string

// Danh mục folder của bucket: folder gốc và folder con cấp một được phép ghi.
// Cấp sâu hơn (ví dụ exams/toeic/audio) do service sở hữu tự đặt.
//
//	topics/{lessons,transcripts}   video và phụ đề bài học (knowledge)
//	avatars/{users,classrooms}     ảnh đại diện user (identity) và ảnh lớp học (learning)
//	exams/{toeic,thpt}             media đề thi (learning)
//	posts/{community,classrooms}   file đính kèm bài viết cộng đồng và bài viết trong lớp (posts)
const (
	FolderTopics  BucketFolder = "topics"
	FolderAvatars BucketFolder = "avatars"
	FolderExams   BucketFolder = "exams"
	FolderPosts   BucketFolder = "posts"

	FolderLessons     BucketFolder = "lessons"
	FolderTranscripts BucketFolder = "transcripts"
	FolderUsers       BucketFolder = "users"
	FolderClassrooms  BucketFolder = "classrooms"
	FolderToeic       BucketFolder = "toeic"
	FolderThpt        BucketFolder = "thpt"
	FolderCommunity   BucketFolder = "community"
)

var folderCatalog = map[BucketFolder][]BucketFolder{
	FolderTopics:  {FolderLessons, FolderTranscripts},
	FolderAvatars: {FolderUsers, FolderClassrooms},
	FolderExams:   {FolderToeic, FolderThpt},
	FolderPosts:   {FolderCommunity, FolderClassrooms},
}

// ErrInvalidFolder: đường dẫn folder không nằm trong danh mục.
var ErrInvalidFolder = errors.New("[MINIO] Folder is not in the catalog")

// ValidateUploadFolders kiểm tra folder trước khi cấp URL ghi: phải có ít nhất folder gốc và folder con,
// cả hai nằm trong danh mục, và không segment nào là "." hoặc "..". Chỉ áp dụng cho thao tác ghi; đọc và xoá vẫn nhận key cũ ngoài danh mục.
func ValidateUploadFolders(folders []BucketFolder) error {
	if len(folders) < 2 {
		return ErrInvalidFolder
	}
	for _, f := range folders {
		if seg := strings.Trim(string(f), "/"); seg == "." || seg == ".." {
			return ErrInvalidFolder
		}
	}
	subs, ok := folderCatalog[BucketFolder(strings.Trim(string(folders[0]), "/"))]
	if !ok || !slices.Contains(subs, BucketFolder(strings.Trim(string(folders[1]), "/"))) {
		return ErrInvalidFolder
	}
	return nil
}

// BuildObjectKey ghép folders và fileName thành object key; trả "" khi tên file rỗng
// để bên gọi trả ErrInvalidKey.
func BuildObjectKey(folders []BucketFolder, fileName string) string {
	if strings.TrimSpace(fileName) == "" {
		return ""
	}
	var parts []string
	for _, f := range folders {
		folderStr := strings.Trim(string(f), "/")
		if folderStr != "" {
			parts = append(parts, folderStr)
		}
	}
	parts = append(parts, fileName)
	return strings.Join(parts, "/")
}
