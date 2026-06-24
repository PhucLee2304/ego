package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	storageClient "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/platform/rpc"
	"ego/services/topics/config"
	"ego/services/topics/database"
	"ego/services/topics/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

const skippedTopicID uint = 11

type topicRow struct {
	csvID   uint
	name    string
	isAudio bool
	url     string
}

type sectionRow struct {
	csvID      uint
	topicCSVID uint
	name       string
}

type lessonRow struct {
	csvID          uint
	sectionCSVID   uint
	title          string
	description    string
	subtitle       string
	localMediaPath string
}

type transcriptRow struct {
	csvID          uint
	lessonCSVID    uint
	order          uint
	content        string
	timeStart      float64
	timeEnd        float64
	localAudioPath string
}

type seedData struct {
	topics      []topicRow
	sections    []sectionRow
	lessons     []lessonRow
	transcripts []transcriptRow
}

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

func main() {
	logger.Setup(logger.LoggerConfig{
		Level:  "debug",
		Pretty: true,
	})

	ctx := context.Background()

	appConfig, err := config.LoadAppConfig()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to load config")
	}

	db, err := database.Connect(appConfig)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to connect to database")
	}

	if err := database.Migrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to migrate database")
	}

	conn, err := grpc.NewClient(appConfig.StorageServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(20*time.Second)))
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to connect to storage service")
	}
	defer conn.Close()
	storage := storageClient.NewStorageServiceClient(conn)

	data, err := loadSeedData(appConfig.SeedDir)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to load seed data")
	}

	jobs, err := buildUploadJobs(appConfig.SeedDir, data)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to validate media files")
	}

	logger.Log.Info().
		Int("topics", len(data.topics)).
		Int("sections", len(data.sections)).
		Int("lessons", len(data.lessons)).
		Int("transcripts", len(data.transcripts)).
		Int("uploads", len(jobs)).
		Int("workers", appConfig.SeedUploadWorkers).
		Int("retries", appConfig.SeedUploadRetries).
		Msg("[SEED] Loaded seed data")

	uploads, err := uploadMedia(ctx, storage, jobs, appConfig.SeedUploadWorkers, appConfig.SeedUploadRetries)
	if err != nil {
		cleanupTopics(ctx, storage, err)
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to upload seed media")
	}

	if err := importData(db, data, uploads); err != nil {
		cleanupTopics(ctx, storage, err)
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to import seed data")
	}

	logger.Log.Info().Msg("[SEED] Topics seed completed")
}

func cleanupTopics(ctx context.Context, storage storageClient.StorageServiceClient, cause error) {
	logger.Log.Error().Err(cause).Msg("[SEED] Cleaning up topics folder")
	cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if _, err := storage.DeleteFolder(cleanupCtx, &storageClient.DeleteFolderRequest{Folder: "topics"}); err != nil {
		logger.Log.Error().Err(err).Msg("[SEED] Failed to cleanup topics folder")
	}
}

func loadSeedData(seedDir string) (*seedData, error) {
	topics, err := loadTopics(filepath.Join(seedDir, "db_topics.csv"))
	if err != nil {
		return nil, err
	}

	sections, skippedSections, err := loadSections(filepath.Join(seedDir, "db_sections.csv"))
	if err != nil {
		return nil, err
	}

	lessons, skippedLessons, err := loadLessons(filepath.Join(seedDir, "db_lessons.csv"), skippedSections)
	if err != nil {
		return nil, err
	}

	transcripts, err := loadTranscripts(filepath.Join(seedDir, "db_transcripts.csv"), skippedLessons)
	if err != nil {
		return nil, err
	}

	return &seedData{
		topics:      topics,
		sections:    sections,
		lessons:     lessons,
		transcripts: transcripts,
	}, nil
}

func loadTopics(path string) ([]topicRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	topics := make([]topicRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "topic id")
		if err != nil {
			return nil, err
		}
		if id == skippedTopicID {
			continue
		}

		isAudio, err := strconv.ParseBool(record["is_audio"])
		if err != nil {
			return nil, fmt.Errorf("invalid topic is_audio for id %d: %w", id, err)
		}

		topics = append(topics, topicRow{
			csvID:   id,
			name:    record["name"],
			isAudio: isAudio,
			url:     record["url"],
		})
	}

	return topics, nil
}

func loadSections(path string) ([]sectionRow, map[uint]struct{}, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, nil, err
	}

	sections := make([]sectionRow, 0, len(records))
	skipped := make(map[uint]struct{})
	for _, record := range records {
		id, err := parseUint(record["id"], "section id")
		if err != nil {
			return nil, nil, err
		}
		topicID, err := parseUint(record["topic_id"], "section topic_id")
		if err != nil {
			return nil, nil, err
		}
		if topicID == skippedTopicID {
			skipped[id] = struct{}{}
			continue
		}

		sections = append(sections, sectionRow{
			csvID:      id,
			topicCSVID: topicID,
			name:       record["name"],
		})
	}

	return sections, skipped, nil
}

func loadLessons(path string, skippedSections map[uint]struct{}) ([]lessonRow, map[uint]struct{}, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, nil, err
	}

	lessons := make([]lessonRow, 0, len(records))
	skipped := make(map[uint]struct{})
	for _, record := range records {
		id, err := parseUint(record["id"], "lesson id")
		if err != nil {
			return nil, nil, err
		}
		sectionID, err := parseUint(record["section_id"], "lesson section_id")
		if err != nil {
			return nil, nil, err
		}
		if _, ok := skippedSections[sectionID]; ok {
			skipped[id] = struct{}{}
			continue
		}

		lessons = append(lessons, lessonRow{
			csvID:          id,
			sectionCSVID:   sectionID,
			title:          record["title"],
			description:    record["description"],
			subtitle:       record["subtitle"],
			localMediaPath: strings.TrimSpace(record["local_full_media_path"]),
		})
	}

	return lessons, skipped, nil
}

func loadTranscripts(path string, skippedLessons map[uint]struct{}) ([]transcriptRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	transcripts := make([]transcriptRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "transcript id")
		if err != nil {
			return nil, err
		}
		lessonID, err := parseUint(record["lesson_id"], "transcript lesson_id")
		if err != nil {
			return nil, err
		}
		if _, ok := skippedLessons[lessonID]; ok {
			continue
		}

		order, err := parseUint(record["sequence_order"], "transcript sequence_order")
		if err != nil {
			return nil, err
		}
		timeStart, err := parseFloat(record["time_start"], "transcript time_start")
		if err != nil {
			return nil, err
		}
		timeEnd, err := parseFloat(record["time_end"], "transcript time_end")
		if err != nil {
			return nil, err
		}

		transcripts = append(transcripts, transcriptRow{
			csvID:          id,
			lessonCSVID:    lessonID,
			order:          order,
			content:        record["content"],
			timeStart:      timeStart,
			timeEnd:        timeEnd,
			localAudioPath: strings.TrimSpace(record["local_audio_path"]),
		})
	}

	return transcripts, nil
}

func readCSV(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header %s: %w", path, err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}

	var records []map[string]string
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row %s: %w", path, err)
		}

		record := make(map[string]string, len(headers))
		for i, header := range headers {
			if i < len(row) {
				record[header] = strings.TrimSpace(row[i])
			}
		}
		records = append(records, record)
	}

	return records, nil
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

func importData(db *gorm.DB, data *seedData, uploads *uploadResult) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := resetTopics(tx); err != nil {
			return err
		}

		topicIDs := make(map[uint]uint, len(data.topics))
		for _, row := range data.topics {
			topic := model.Topic{
				Name:    row.name,
				IsAudio: row.isAudio,
				Url:     row.url,
			}
			if err := tx.Create(&topic).Error; err != nil {
				return fmt.Errorf("create topic csv id %d: %w", row.csvID, err)
			}
			topicIDs[row.csvID] = topic.ID
		}

		sectionIDs := make(map[uint]uint, len(data.sections))
		for _, row := range data.sections {
			topicID, ok := topicIDs[row.topicCSVID]
			if !ok {
				return fmt.Errorf("missing topic db id for section csv id %d topic csv id %d", row.csvID, row.topicCSVID)
			}

			section := model.Section{
				Name:    row.name,
				TopicID: topicID,
			}
			if err := tx.Create(&section).Error; err != nil {
				return fmt.Errorf("create section csv id %d: %w", row.csvID, err)
			}
			sectionIDs[row.csvID] = section.ID
		}

		lessonIDs := make(map[uint]uint, len(data.lessons))
		for _, row := range data.lessons {
			sectionID, ok := sectionIDs[row.sectionCSVID]
			if !ok {
				return fmt.Errorf("missing section db id for lesson csv id %d section csv id %d", row.csvID, row.sectionCSVID)
			}

			url, ok := uploads.lessonURLs[row.csvID]
			if !ok {
				return fmt.Errorf("missing uploaded lesson url for lesson csv id %d", row.csvID)
			}

			lesson := model.Lesson{
				Title:       row.title,
				Description: row.description,
				Subtitle:    row.subtitle,
				Url:         url,
				SectionID:   sectionID,
			}
			if err := tx.Create(&lesson).Error; err != nil {
				return fmt.Errorf("create lesson csv id %d: %w", row.csvID, err)
			}
			lessonIDs[row.csvID] = lesson.ID
		}

		transcripts := make([]model.Transcript, 0, len(data.transcripts))
		for _, row := range data.transcripts {
			lessonID, ok := lessonIDs[row.lessonCSVID]
			if !ok {
				return fmt.Errorf("missing lesson db id for transcript csv id %d lesson csv id %d", row.csvID, row.lessonCSVID)
			}

			transcripts = append(transcripts, model.Transcript{
				Content:   row.content,
				Order:     row.order,
				TimeStart: row.timeStart,
				TimeEnd:   row.timeEnd,
				Url:       uploads.transcriptURLs[row.csvID],
				LessonID:  lessonID,
			})
		}
		if len(transcripts) > 0 {
			if err := tx.CreateInBatches(transcripts, 1000).Error; err != nil {
				return fmt.Errorf("create transcripts: %w", err)
			}
		}

		return nil
	})
}

func resetTopics(tx *gorm.DB) error {
	models := []any{
		&model.Transcript{},
		&model.Lesson{},
		&model.Section{},
		&model.Topic{},
	}

	for _, m := range models {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(m).Error; err != nil {
			return fmt.Errorf("reset topics data: %w", err)
		}
	}

	return nil
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

func parseUint(value, name string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return uint(parsed), nil
}

func parseFloat(value, name string) (float64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}
