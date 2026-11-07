package service

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"testing"

	"ego/services/storage/internal/dto"
	"ego/services/storage/minio"
)

func TestGeneratePresignUploadURLFolders(t *testing.T) {
	tests := []struct {
		folder string
		want   int // 0: thành công
		key    string
	}{
		{folder: "avatars/users", key: "avatars/users/"},
		{folder: "posts/community", key: "posts/community/"},
		{folder: "exams/toeic", key: "exams/toeic/"},
		{folder: "exams/toeic/audio", want: http.StatusBadRequest},
		{folder: "avatars/users/../../exams", want: http.StatusBadRequest},
		{folder: "avatars", want: http.StatusBadRequest},
		{folder: "posts/uploads", want: http.StatusBadRequest},
		{folder: "avatars//users", want: http.StatusBadRequest},
		{folder: "../avatars/users", want: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.folder, func(t *testing.T) {
			store := &fakeStore{}
			resp, err := New(store).GeneratePresignUploadURL(context.Background(), dto.PresignUploadRequest{Folder: tt.folder, FileName: "Lớp 12A.png", ContentType: "image/png"})
			if tt.want != 0 {
				if err == nil || err.StatusCode != tt.want {
					t.Fatalf("want %d, got %v", tt.want, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := minio.BuildObjectKey(store.folders, "x"); got != tt.key+"x" {
				t.Fatalf("folder = %q, want %q", got, tt.key)
			}
			if resp.FileName != store.fileName || !regexp.MustCompile(`^\d{13}-lop-12a-[a-z0-9]{6}\.png$`).MatchString(resp.FileName) {
				t.Fatalf("fileName = %q, store got %q", resp.FileName, store.fileName)
			}
			if store.internal {
				t.Fatal("client upload must not use the internal endpoint")
			}
		})
	}
}

// fakeStore ghi lại tham số của lần presign upload gần nhất.
type fakeStore struct {
	ObjectStore
	folders  []minio.BucketFolder
	fileName string
	internal bool
}

func (f *fakeStore) GeneratePresignedUploadURL(_ context.Context, folders []minio.BucketFolder, fileName, _ string, internal bool) (*string, error) {
	if err := minio.ValidateUploadFolders(folders); err != nil {
		return nil, err
	}
	f.folders, f.fileName, f.internal = folders, fileName, internal
	url := "https://example/" + fileName
	return &url, nil
}

func TestPresignUploadKeepsFileName(t *testing.T) {
	store := &fakeStore{}
	if _, err := New(store).PresignUpload(context.Background(), []string{"topics", "lessons"}, "abc123.mp4", "video/mp4", true); err != nil {
		t.Fatal(err)
	}
	if store.fileName != "abc123.mp4" || !store.internal {
		t.Fatalf("got fileName=%q internal=%v", store.fileName, store.internal)
	}
	if _, err := New(store).PresignUpload(context.Background(), []string{"posts", "uploads"}, "a.png", "image/png", false); !errors.Is(err, minio.ErrInvalidFolder) {
		t.Fatalf("want ErrInvalidFolder, got %v", err)
	}
}
