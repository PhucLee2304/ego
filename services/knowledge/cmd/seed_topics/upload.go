package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	storageClient "ego/api/gen/go/storage"
	"ego/platform/logger"
)

type uploadKind string

const (
	uploadLesson     uploadKind = "lesson"
	uploadTranscript uploadKind = "transcript"
)

type uploadJob struct {
	kind        uploadKind
	csvID       uint
	localPath   string
	folders     []string
	fileName    string
	contentType string
}

type uploadResult struct {
	lessonURLs     map[uint]string
	transcriptURLs map[uint]string
}

func buildUploadJobs(seedDir string, data *seedData) ([]uploadJob, error) {
	var jobs []uploadJob

	for _, lesson := range data.lessons {
		if lesson.localMediaPath == "" {
			return nil, fmt.Errorf("lesson csv id %d has empty local_full_media_path", lesson.csvID)
		}

		localPath := resolveSeedPath(seedDir, lesson.localMediaPath)
		if err := requireFile(localPath); err != nil {
			return nil, fmt.Errorf("lesson csv id %d: %w", lesson.csvID, err)
		}

		fileName := filepath.Base(localPath)
		jobs = append(jobs, uploadJob{
			kind:        uploadLesson,
			csvID:       lesson.csvID,
			localPath:   localPath,
			folders:     []string{"topics", "lessons"},
			fileName:    fileName,
			contentType: contentType(fileName),
		})
	}

	for _, transcript := range data.transcripts {
		if transcript.localAudioPath == "" {
			continue
		}

		localPath := resolveSeedPath(seedDir, transcript.localAudioPath)
		if err := requireFile(localPath); err != nil {
			return nil, fmt.Errorf("transcript csv id %d: %w", transcript.csvID, err)
		}

		fileName := filepath.Base(localPath)
		jobs = append(jobs, uploadJob{
			kind:        uploadTranscript,
			csvID:       transcript.csvID,
			localPath:   localPath,
			folders:     []string{"topics", "transcripts"},
			fileName:    fileName,
			contentType: contentType(fileName),
		})
	}

	return jobs, nil
}

func uploadMedia(ctx context.Context, storage storageClient.StorageServiceClient, jobs []uploadJob, workers int, retries int) (*uploadResult, error) {
	if workers <= 0 {
		workers = 8
	}
	if retries <= 0 {
		retries = 5
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	result := &uploadResult{
		lessonURLs:     make(map[uint]string),
		transcriptURLs: make(map[uint]string),
	}

	httpClient := &http.Client{Timeout: 30 * time.Minute}
	jobCh := make(chan uploadJob)
	errCh := make(chan error, 1)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}

				url, err := uploadOneWithRetry(ctx, httpClient, storage, job, retries)
				if err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}

				mu.Lock()
				switch job.kind {
				case uploadLesson:
					result.lessonURLs[job.csvID] = url
				case uploadTranscript:
					result.transcriptURLs[job.csvID] = url
				}
				mu.Unlock()
			}
		}()
	}

sendJobs:
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			break sendJobs
		case jobCh <- job:
		}
		if ctx.Err() != nil {
			break sendJobs
		}
	}
	close(jobCh)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	if len(result.lessonURLs)+len(result.transcriptURLs) != len(jobs) {
		return nil, fmt.Errorf("uploaded %d/%d media files", len(result.lessonURLs)+len(result.transcriptURLs), len(jobs))
	}

	return result, nil
}

func uploadOneWithRetry(ctx context.Context, httpClient *http.Client, storage storageClient.StorageServiceClient, job uploadJob, retries int) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= retries+1; attempt++ {
		url, retryable, err := uploadOne(ctx, httpClient, storage, job)
		if err == nil {
			return url, nil
		}
		lastErr = err
		if !retryable || attempt > retries {
			break
		}

		delay := time.Duration(attempt*attempt) * time.Second
		logger.Log.Warn().
			Err(err).
			Str("file", job.localPath).
			Int("attempt", attempt).
			Int("max_retries", retries).
			Dur("retry_after", delay).
			Msg("[SEED] Retrying media upload")

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}

	return "", lastErr
}

func uploadOne(ctx context.Context, httpClient *http.Client, storage storageClient.StorageServiceClient, job uploadJob) (string, bool, error) {
	presign, err := storage.GeneratePresignedUploadURL(ctx, &storageClient.GeneratePresignedUploadURLRequest{
		Folders:     job.folders,
		FileName:    job.fileName,
		ContentType: job.contentType,
	})
	if err != nil {
		return "", true, fmt.Errorf("generate presigned upload url for %s: %w", job.localPath, err)
	}

	file, err := os.Open(job.localPath)
	if err != nil {
		return "", false, fmt.Errorf("open file %s: %w", job.localPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", false, fmt.Errorf("stat file %s: %w", job.localPath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presign.Url, file)
	if err != nil {
		return "", false, fmt.Errorf("create upload request for %s: %w", job.localPath, err)
	}
	req.Header.Set("Content-Type", job.contentType)
	req.ContentLength = info.Size()

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("upload file %s: %w", job.localPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", isRetryableStatus(resp.StatusCode), fmt.Errorf("upload file %s failed with status %s: %s", job.localPath, resp.Status, string(body))
	}

	publicURL, err := storage.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
		Folders:  job.folders,
		FileName: job.fileName,
	})
	if err != nil {
		return "", true, fmt.Errorf("get public url for %s: %w", job.localPath, err)
	}

	return publicURL.Url, false, nil
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}

func resolveSeedPath(seedDir, localPath string) string {
	cleanPath := filepath.FromSlash(strings.TrimSpace(localPath))
	if filepath.IsAbs(cleanPath) {
		return cleanPath
	}
	return filepath.Join(seedDir, cleanPath)
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing media file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("media path is a directory: %s", path)
	}
	return nil
}

func contentType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	default:
		return "application/octet-stream"
	}
}
