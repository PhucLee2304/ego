package minio

import "testing"

func TestValidateUploadFolders(t *testing.T) {
	tests := []struct {
		folders []BucketFolder
		ok      bool
	}{
		{[]BucketFolder{"topics", "lessons"}, true},
		{[]BucketFolder{"exams", "toeic", "audio"}, true},
		{[]BucketFolder{"posts", "classrooms"}, true},
		{[]BucketFolder{"avatars", "users"}, true},
		{[]BucketFolder{"avatars", "users", "..", "..", "exams"}, false},
		{[]BucketFolder{"avatars"}, false},
		{[]BucketFolder{"posts", "uploads", "1"}, false},
		{[]BucketFolder{"posts", "videos", "1"}, false},
		{[]BucketFolder{"topic", "lessons"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if err := ValidateUploadFolders(tt.folders); (err == nil) != tt.ok {
			t.Errorf("ValidateUploadFolders(%v) = %v, want ok=%v", tt.folders, err, tt.ok)
		}
	}
}

func TestBuildObjectKeyEmptyFileName(t *testing.T) {
	for _, name := range []string{"", "  "} {
		if got := BuildObjectKey([]BucketFolder{"posts", "community"}, name); got != "" {
			t.Fatalf("BuildObjectKey(%q) = %q, want empty", name, got)
		}
	}
}
