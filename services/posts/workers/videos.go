package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	storage "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
	"github.com/google/uuid"
)

type probeResult struct {
	Streams []struct {
		Width    int    `json:"width"`
		Height   int    `json:"height"`
		Duration string `json:"duration"`
	} `json:"streams"`
}

func StartVideoProcessor(ctx context.Context, repo *repository.Repository, storageClient storage.StorageServiceClient, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processVideos(ctx, repo, storageClient)
			}
		}
	}()
}

func processVideos(ctx context.Context, repo *repository.Repository, storageClient storage.StorageServiceClient) {
	items, err := repo.ListVideoAttachmentsForProcessing(ctx, 5)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[POSTS] Failed to list videos")
		return
	}
	for _, item := range items {
		item.ProcessingAttempts++
		if err := processVideo(ctx, repo, storageClient, item); err != nil {
			if item.ProcessingAttempts >= 3 {
				item.ProcessingStatus = model.ProcessingStatusFailed
			}
			_ = repo.SaveAttachment(ctx, item)
			logger.Log.Error().Err(err).Uint("attachment", item.ID).Msg("[POSTS] Video processing failed")
		}
	}
}

func processVideo(ctx context.Context, repo *repository.Repository, storageClient storage.StorageServiceClient, item *model.PostAttachment) error {
	dir, err := os.MkdirTemp("", "post-video-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	input, output, thumbnail := filepath.Join(dir, "input"), filepath.Join(dir, "output.mp4"), filepath.Join(dir, "thumbnail.jpg")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download status %s", response.Status)
	}
	file, err := os.Create(input)
	if err != nil {
		return err
	}
	if _, err = file.ReadFrom(response.Body); err != nil {
		file.Close()
		return err
	}
	file.Close()
	probeOutput, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,duration", "-of", "json", input).Output()
	if err != nil {
		return err
	}
	var probe probeResult
	if err := json.Unmarshal(probeOutput, &probe); err != nil || len(probe.Streams) == 0 {
		return fmt.Errorf("invalid video metadata")
	}
	if err := exec.CommandContext(ctx, "ffmpeg", "-y", "-i", input, "-vf", "scale=-2:min(720\\,ih)", "-c:v", "libx264", "-preset", "fast", "-crf", "23", "-c:a", "aac", "-movflags", "+faststart", output).Run(); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "0", "-i", output, "-frames:v", "1", "-q:v", "3", thumbnail).Run(); err != nil {
		return err
	}
	folder := []string{"posts", "videos", item.UserID}
	videoName, thumbName := uuid.NewString()+".mp4", uuid.NewString()+".jpg"
	videoURL, err := uploadProcessed(ctx, storageClient, folder, videoName, "video/mp4", output)
	if err != nil {
		return err
	}
	thumbURL, err := uploadProcessed(ctx, storageClient, folder, thumbName, "image/jpeg", thumbnail)
	if err != nil {
		return err
	}
	parts := strings.Split(strings.Trim(item.ObjectKey, "/"), "/")
	if len(parts) > 1 {
		_, _ = storageClient.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: parts[:len(parts)-1], FileName: parts[len(parts)-1]})
	}
	duration, _ := strconv.ParseFloat(probe.Streams[0].Duration, 64)
	sizeInfo, _ := os.Stat(output)
	item.URL, item.ObjectKey, item.ContentType = videoURL, strings.Join(append(folder, videoName), "/"), "video/mp4"
	item.Size, item.Width, item.Height, item.DurationSeconds, item.ThumbnailURL = sizeInfo.Size(), &probe.Streams[0].Width, &probe.Streams[0].Height, &duration, &thumbURL
	item.ProcessingStatus = model.ProcessingStatusReady
	return repo.SaveAttachment(ctx, item)
}

func uploadProcessed(ctx context.Context, client storage.StorageServiceClient, folders []string, name, contentType, path string) (string, error) {
	presigned, err := client.GeneratePresignedUploadURL(ctx, &storage.GeneratePresignedUploadURLRequest{Folders: folders, FileName: name, ContentType: contentType})
	if err != nil {
		return "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, _ := file.Stat()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, presigned.Url, file)
	req.Header.Set("Content-Type", contentType)
	req.ContentLength = info.Size()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload status %s", resp.Status)
	}
	public, err := client.GetPublicURL(ctx, &storage.GetPublicURLRequest{Folders: folders, FileName: name})
	if err != nil {
		return "", err
	}
	return public.Url, nil
}
