package minio

import (
	"strings"
)

type BucketFolder string

const (
	FolderTopics      BucketFolder = "topics"
	FolderLessons     BucketFolder = "lessons"
	FolderTranscripts BucketFolder = "transcripts"
	FolderAvatars     BucketFolder = "avatars"
)

func BuildObjectKey(folders []BucketFolder, fileName string) string {
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
