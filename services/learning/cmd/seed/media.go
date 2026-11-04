package main

import (
	"context"
	"encoding/json"
	"errors"
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

func loadManifest(path string) (*mediaManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest %s: %w", path, err)
	}
	defer file.Close()

	var manifest mediaManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", path, err)
	}

	return &manifest, nil
}

func verifyManifest(seedDir string, data *seedData) error {
	if data == nil || data.manifest == nil {
		return errors.New("manifest is nil")
	}

	manifestByURL := make(map[string]mediaEntry, len(data.manifest.Entries))
	for _, entry := range data.manifest.Entries {
		if entry.Status != "downloaded" && entry.Status != "skipped" {
			return fmt.Errorf("media entry %s is not ready: status=%s error=%s", entry.SourceURL, entry.Status, entry.Error)
		}

		localPath := filepath.Join(seedDir, filepath.FromSlash(entry.LocalPath))
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("missing media file %s: %w", localPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("media path is a directory: %s", localPath)
		}
		if info.Size() <= 0 {
			return fmt.Errorf("media file is empty: %s", localPath)
		}

		manifestByURL[entry.SourceURL] = entry
	}

	for _, row := range data.toeicQuestions {
		if row.AudioURL != "" {
			if _, ok := manifestByURL[row.AudioURL]; !ok {
				return fmt.Errorf("missing audio url in manifest for question %s: %s", row.QuestionID, row.AudioURL)
			}
		}
		for _, imageURL := range row.ImageURLs {
			if _, ok := manifestByURL[imageURL]; !ok {
				return fmt.Errorf("missing image url in manifest for question %s: %s", row.QuestionID, imageURL)
			}
		}
	}

	return nil
}

func buildUploadJobs(seedDir string, manifest *mediaManifest) ([]uploadJob, error) {
	if manifest == nil {
		return nil, errors.New("manifest is nil")
	}

	jobs := make([]uploadJob, 0, len(manifest.Entries))
	for _, entry := range manifest.Entries {
		localPath := filepath.Join(seedDir, filepath.FromSlash(entry.LocalPath))
		if err := requireFile(localPath); err != nil {
			return nil, err
		}

		folders := []string{examsRootFolder, examsToeicFolder}
		if entry.Type == examsToeicImageType {
			folders = []string{examsRootFolder, examsToeicFolder, examsToeicImageFolder}
		} else if entry.Type == examsToeicAudioType {
			folders = []string{examsRootFolder, examsToeicFolder, examsToeicAudioFolder}
		}

		jobs = append(jobs, uploadJob{
			SourceURL:   entry.SourceURL,
			LocalPath:   localPath,
			FileName:    entry.Filename,
			Folders:     folders,
			ContentType: contentType(entry.Filename),
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
		PublicURLs: make(map[string]string, len(jobs)),
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

				publicURL, err := uploadOneWithRetry(ctx, httpClient, storage, job, retries)
				if err != nil {
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}

				mu.Lock()
				result.PublicURLs[job.SourceURL] = publicURL
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
	}

	close(jobCh)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}

	if len(result.PublicURLs) != len(jobs) {
		return nil, fmt.Errorf("uploaded %d/%d media files", len(result.PublicURLs), len(jobs))
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
			Str("file", job.LocalPath).
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
		Folders:     job.Folders,
		FileName:    job.FileName,
		ContentType: job.ContentType,
	})
	if err != nil {
		return "", true, fmt.Errorf("generate presigned upload url for %s: %w", job.LocalPath, err)
	}

	file, err := os.Open(job.LocalPath)
	if err != nil {
		return "", false, fmt.Errorf("open file %s: %w", job.LocalPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", false, fmt.Errorf("stat file %s: %w", job.LocalPath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presign.Url, file)
	if err != nil {
		return "", false, fmt.Errorf("create upload request for %s: %w", job.LocalPath, err)
	}
	req.Header.Set("Content-Type", job.ContentType)
	req.ContentLength = info.Size()

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("upload file %s: %w", job.LocalPath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", isRetryableStatus(resp.StatusCode), fmt.Errorf("upload file %s failed with status %s: %s", job.LocalPath, resp.Status, string(body))
	}

	publicURL, err := storage.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
		Folders:  job.Folders,
		FileName: job.FileName,
	})
	if err != nil {
		return "", true, fmt.Errorf("get public url for %s: %w", job.LocalPath, err)
	}

	return publicURL.Url, false, nil
}

func toPublicURLPointer(uploads *uploadResult, sourceURL string) *string {
	sourceURL = strings.TrimSpace(sourceURL)
	if uploads == nil || sourceURL == "" {
		return nil
	}

	url, ok := uploads.PublicURLs[sourceURL]
	if !ok || strings.TrimSpace(url) == "" {
		return nil
	}
	return &url
}

func contentType(fileName string) string {
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".mp4":
		return "video/mp4"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}
