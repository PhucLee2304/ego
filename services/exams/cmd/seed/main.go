package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	storageClient "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/platform/rpc"
	"ego/services/exams/config"
	"ego/services/exams/database"
	"ego/services/exams/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

const (
	thptSectionTitle      = "Full"
	examsRootFolder       = "exams"
	examsToeicFolder      = "toeic"
	examsToeicAudioFolder = "audio"
	examsToeicImageFolder = "images"
	examsToeicAudioType   = "audio"
	examsToeicImageType   = "image"
	defaultTHPTDuration   = 60
	defaultListenDuration = 45
	defaultReadDuration   = 75
)

var (
	yearPattern         = regexp.MustCompile(`(?i)(19|20)\d{2}`)
	transcriptOptionExp = regexp.MustCompile(`\(([A-D])\)`)
)

type thptTestRow struct {
	ID             uint
	Title          string
	TotalQuestions int
}

type thptQuestionRow struct {
	ID            uint
	TestID        uint
	QuestionNo    int
	Content       string
	CorrectAnswer string
	RowOrder      int
}

type thptAnswerRow struct {
	ID         uint
	QuestionID uint
	Value      string
	Text       string
}

type toeicTestRow struct {
	TestSetID      string
	Slug           string
	Title          string
	TotalQuestions int
}

type toeicQuestionRow struct {
	QuestionID         string
	TestSetID          string
	GroupID            string
	Section            string
	Part               string
	GroupOrder         int
	QuestionNo         int
	QuestionText       string
	PassageText        string
	AudioTranscript    string
	Explanation        string
	CorrectAnswerOrder int
	ImageURL           string
	AudioURL           string
	RowOrder           int
}

type toeicAnswerRow struct {
	AnswerID    string
	QuestionID  string
	AnswerOrder int
	Content     string
	IsCorrect   bool
}

type mediaEntry struct {
	Type      string `json:"type"`
	SourceURL string `json:"source_url"`
	LocalPath string `json:"local_path"`
	Filename  string `json:"filename"`
	GroupID   string `json:"group_id"`
	Part      string `json:"part"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	Size      int64  `json:"size"`
}

type mediaManifest struct {
	Entries []mediaEntry `json:"entries"`
}

type seedData struct {
	thptTests      []thptTestRow
	thptQuestions  []thptQuestionRow
	thptAnswers    []thptAnswerRow
	toeicTests     []toeicTestRow
	toeicQuestions []toeicQuestionRow
	toeicAnswers   []toeicAnswerRow
	manifest       *mediaManifest
}

type uploadJob struct {
	SourceURL   string
	LocalPath   string
	FileName    string
	Folders     []string
	ContentType string
}

type uploadResult struct {
	PublicURLs map[string]string
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

	conn, err := grpc.NewClient(
		appConfig.StorageServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.TimeoutInterceptor(20*time.Second)),
	)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to connect to storage service")
	}
	defer conn.Close()

	storage := storageClient.NewStorageServiceClient(conn)

	data, err := loadSeedData(appConfig.SeedDir)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to load seed data")
	}

	jobs, err := buildUploadJobs(appConfig.SeedDir, data.manifest)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to prepare upload jobs")
	}

	logger.Log.Info().
		Int("thpt_tests", len(data.thptTests)).
		Int("thpt_questions", len(data.thptQuestions)).
		Int("thpt_answers", len(data.thptAnswers)).
		Int("toeic_tests", len(data.toeicTests)).
		Int("toeic_questions", len(data.toeicQuestions)).
		Int("toeic_answers", len(data.toeicAnswers)).
		Int("uploads", len(jobs)).
		Int("workers", appConfig.SeedUploadWorkers).
		Int("retries", appConfig.SeedUploadRetries).
		Msg("[SEED] Loaded exams seed data")

	uploads, err := uploadMedia(ctx, storage, jobs, appConfig.SeedUploadWorkers, appConfig.SeedUploadRetries)
	if err != nil {
		cleanupExams(ctx, storage, err)
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to upload exam media")
	}

	if err := importData(db, data, uploads); err != nil {
		cleanupExams(ctx, storage, err)
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to import exam data")
	}

	logger.Log.Info().Msg("[SEED] Exams seed completed")
}

func cleanupExams(ctx context.Context, storage storageClient.StorageServiceClient, cause error) {
	logger.Log.Error().Err(cause).Msg("[SEED] Cleaning up exams folder")
	cleanupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if _, err := storage.DeleteFolder(cleanupCtx, &storageClient.DeleteFolderRequest{
		Folder: examsRootFolder,
	}); err != nil {
		logger.Log.Error().Err(err).Msg("[SEED] Failed to cleanup exams folder")
	}
}

func loadSeedData(seedDir string) (*seedData, error) {
	thptTests, err := loadTHPTTests(filepath.Join(seedDir, "db", "tests.csv"))
	if err != nil {
		return nil, err
	}

	thptQuestions, err := loadTHPTQuestions(filepath.Join(seedDir, "db", "questions.csv"))
	if err != nil {
		return nil, err
	}

	thptAnswers, err := loadTHPTAnswers(filepath.Join(seedDir, "db", "answers.csv"))
	if err != nil {
		return nil, err
	}

	toeicTests, err := loadTOEICTests(filepath.Join(seedDir, "db", "toeic_tests.csv"))
	if err != nil {
		return nil, err
	}

	toeicQuestions, err := loadTOEICQuestions(filepath.Join(seedDir, "db", "toeic_questions.csv"))
	if err != nil {
		return nil, err
	}

	toeicAnswers, err := loadTOEICAnswers(filepath.Join(seedDir, "db", "toeic_answers.csv"))
	if err != nil {
		return nil, err
	}

	manifest, err := loadManifest(filepath.Join(seedDir, "media", "manifests", "toeic_media.json"))
	if err != nil {
		return nil, err
	}

	data := &seedData{
		thptTests:      thptTests,
		thptQuestions:  thptQuestions,
		thptAnswers:    thptAnswers,
		toeicTests:     toeicTests,
		toeicQuestions: toeicQuestions,
		toeicAnswers:   toeicAnswers,
		manifest:       manifest,
	}

	if err := verifyManifest(seedDir, data); err != nil {
		return nil, err
	}

	return data, nil
}

func loadTHPTTests(path string) ([]thptTestRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptTestRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "thpt test id")
		if err != nil {
			return nil, err
		}

		totalQuestions, err := parseInt(record["total_questions"], "thpt total_questions")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptTestRow{
			ID:             id,
			Title:          record["title"],
			TotalQuestions: totalQuestions,
		})
	}

	return rows, nil
}

func loadTHPTQuestions(path string) ([]thptQuestionRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptQuestionRow, 0, len(records))
	for i, record := range records {
		id, err := parseUint(record["id"], "thpt question id")
		if err != nil {
			return nil, err
		}
		testID, err := parseUint(record["test_id"], "thpt question test_id")
		if err != nil {
			return nil, err
		}
		questionNo, err := parseInt(record["question_number"], "thpt question_number")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptQuestionRow{
			ID:            id,
			TestID:        testID,
			QuestionNo:    questionNo,
			Content:       cleanText(record["content"]),
			CorrectAnswer: strings.ToUpper(strings.TrimSpace(record["correct_answer"])),
			RowOrder:      i + 1,
		})
	}

	return rows, nil
}

func loadTHPTAnswers(path string) ([]thptAnswerRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]thptAnswerRow, 0, len(records))
	for _, record := range records {
		id, err := parseUint(record["id"], "thpt answer id")
		if err != nil {
			return nil, err
		}
		questionID, err := parseUint(record["question_id"], "thpt answer question_id")
		if err != nil {
			return nil, err
		}

		rows = append(rows, thptAnswerRow{
			ID:         id,
			QuestionID: questionID,
			Value:      strings.ToUpper(strings.TrimSpace(record["answer_value"])),
			Text:       cleanText(record["answer_text"]),
		})
	}

	return rows, nil
}

func loadTOEICTests(path string) ([]toeicTestRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicTestRow, 0, len(records))
	for _, record := range records {
		totalQuestions, err := parseInt(record["total_questions"], "toeic total_questions")
		if err != nil {
			return nil, err
		}

		rows = append(rows, toeicTestRow{
			TestSetID:      strings.TrimSpace(record["test_set_id"]),
			Slug:           strings.TrimSpace(record["test_slug"]),
			Title:          cleanText(record["title"]),
			TotalQuestions: totalQuestions,
		})
	}

	return rows, nil
}

func loadTOEICQuestions(path string) ([]toeicQuestionRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicQuestionRow, 0, len(records))
	for i, record := range records {
		groupOrder, err := parseInt(record["group_order"], "toeic group_order")
		if err != nil {
			return nil, err
		}
		questionNo, err := parseInt(record["question_number"], "toeic question_number")
		if err != nil {
			return nil, err
		}
		correctOrder, err := parseInt(record["correct_answer_order"], "toeic correct_answer_order")
		if err != nil {
			return nil, err
		}

		rows = append(rows, toeicQuestionRow{
			QuestionID:         strings.TrimSpace(record["question_id"]),
			TestSetID:          strings.TrimSpace(record["test_set_id"]),
			GroupID:            strings.TrimSpace(record["group_id"]),
			Section:            strings.ToUpper(strings.TrimSpace(record["section"])),
			Part:               strings.ToUpper(strings.TrimSpace(record["part"])),
			GroupOrder:         groupOrder,
			QuestionNo:         questionNo,
			QuestionText:       cleanText(record["question_text"]),
			PassageText:        cleanText(record["passage_text"]),
			AudioTranscript:    cleanText(record["audio_transcript"]),
			Explanation:        cleanText(record["explanation"]),
			CorrectAnswerOrder: correctOrder,
			ImageURL:           strings.TrimSpace(record["image_url"]),
			AudioURL:           strings.TrimSpace(record["audio_url"]),
			RowOrder:           i + 1,
		})
	}

	return rows, nil
}

func loadTOEICAnswers(path string) ([]toeicAnswerRow, error) {
	records, err := readCSV(path)
	if err != nil {
		return nil, err
	}

	rows := make([]toeicAnswerRow, 0, len(records))
	for _, record := range records {
		answerOrder, err := parseInt(record["answer_order"], "toeic answer_order")
		if err != nil {
			return nil, err
		}
		isCorrect, err := strconv.ParseBool(strings.TrimSpace(record["is_correct"]))
		if err != nil {
			return nil, fmt.Errorf("invalid toeic is_correct %q: %w", record["is_correct"], err)
		}

		rows = append(rows, toeicAnswerRow{
			AnswerID:    strings.TrimSpace(record["answer_id"]),
			QuestionID:  strings.TrimSpace(record["question_id"]),
			AnswerOrder: answerOrder,
			Content:     cleanText(record["content"]),
			IsCorrect:   isCorrect,
		})
	}

	return rows, nil
}

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
		if row.ImageURL != "" {
			if _, ok := manifestByURL[row.ImageURL]; !ok {
				return fmt.Errorf("missing image url in manifest for question %s: %s", row.QuestionID, row.ImageURL)
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

func importData(db *gorm.DB, data *seedData, uploads *uploadResult) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := resetExams(tx); err != nil {
			return err
		}

		if err := importTHPT(tx, data); err != nil {
			return err
		}
		if err := importTOEIC(tx, data, uploads); err != nil {
			return err
		}

		return nil
	})
}

func importTHPT(tx *gorm.DB, data *seedData) error {
	questionsByTest := make(map[uint][]thptQuestionRow)
	for _, row := range data.thptQuestions {
		questionsByTest[row.TestID] = append(questionsByTest[row.TestID], row)
	}
	for _, rows := range questionsByTest {
		sort.SliceStable(rows, func(i, j int) bool {
			if rows[i].QuestionNo == rows[j].QuestionNo {
				return rows[i].RowOrder < rows[j].RowOrder
			}
			return rows[i].QuestionNo < rows[j].QuestionNo
		})
	}

	answersByQuestion := make(map[uint][]thptAnswerRow)
	for _, row := range data.thptAnswers {
		answersByQuestion[row.QuestionID] = append(answersByQuestion[row.QuestionID], row)
	}
	for _, rows := range answersByQuestion {
		sort.SliceStable(rows, func(i, j int) bool {
			return optionOrderFromKey(rows[i].Value) < optionOrderFromKey(rows[j].Value)
		})
	}

	for _, test := range data.thptTests {
		exam := model.Exam{
			Title:       test.Title,
			Description: nil,
			IsPublic:    true,
			Type:        model.ExamTypeTHPT,
			Year:        extractYear(test.Title),
		}
		if err := tx.Create(&exam).Error; err != nil {
			return fmt.Errorf("create thpt exam %d: %w", test.ID, err)
		}

		duration := defaultTHPTDuration
		section := model.Section{
			Code:     model.SectionCodeFull,
			Title:    thptSectionTitle,
			Order:    1,
			Duration: &duration,
			ExamID:   exam.ID,
		}
		if err := tx.Create(&section).Error; err != nil {
			return fmt.Errorf("create thpt section for exam %d: %w", test.ID, err)
		}

		options := make([]model.Option, 0, 200)
		for _, row := range questionsByTest[test.ID] {
			question := model.Question{
				Content:     row.Content,
				Explanation: "",
				Order:       row.QuestionNo,
				SectionID:   section.ID,
			}
			if err := tx.Create(&question).Error; err != nil {
				return fmt.Errorf("create thpt question %d: %w", row.ID, err)
			}

			answers := answersByQuestion[row.ID]
			if len(answers) == 0 {
				return fmt.Errorf("thpt question %d has no answers", row.ID)
			}

			for _, answer := range answers {
				key, err := parseOptionKey(answer.Value)
				if err != nil {
					return fmt.Errorf("thpt question %d: %w", row.ID, err)
				}

				options = append(options, model.Option{
					Key:        key,
					Content:    stringPointer(normalizeOptionContent(answer.Value, answer.Text)),
					IsCorrect:  strings.EqualFold(answer.Value, row.CorrectAnswer),
					Order:      optionOrderFromKey(answer.Value),
					QuestionID: question.ID,
				})
			}
		}

		if len(options) > 0 {
			if err := tx.CreateInBatches(options, 1000).Error; err != nil {
				return fmt.Errorf("create thpt options for exam %d: %w", test.ID, err)
			}
		}
	}

	return nil
}

func importTOEIC(tx *gorm.DB, data *seedData, uploads *uploadResult) error {
	questionsByTest := make(map[string][]toeicQuestionRow)
	for _, row := range data.toeicQuestions {
		questionsByTest[row.TestSetID] = append(questionsByTest[row.TestSetID], row)
	}
	for _, rows := range questionsByTest {
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].RowOrder < rows[j].RowOrder
		})
	}

	answersByQuestion := make(map[string][]toeicAnswerRow)
	for _, row := range data.toeicAnswers {
		answersByQuestion[row.QuestionID] = append(answersByQuestion[row.QuestionID], row)
	}
	for _, rows := range answersByQuestion {
		sort.SliceStable(rows, func(i, j int) bool {
			return rows[i].AnswerOrder < rows[j].AnswerOrder
		})
	}

	for _, test := range data.toeicTests {
		exam := model.Exam{
			Title:       test.Title,
			Description: nil,
			IsPublic:    true,
			Type:        model.ExamTypeTOEIC,
			Year:        extractYear(test.Title),
		}
		if err := tx.Create(&exam).Error; err != nil {
			return fmt.Errorf("create toeic exam %s: %w", test.TestSetID, err)
		}

		listenDuration := defaultListenDuration
		readDuration := defaultReadDuration
		listeningSection := model.Section{
			Code:     model.SectionCodeListening,
			Title:    "Listening",
			Order:    1,
			Duration: &listenDuration,
			ExamID:   exam.ID,
		}
		readingSection := model.Section{
			Code:     model.SectionCodeReading,
			Title:    "Reading",
			Order:    2,
			Duration: &readDuration,
			ExamID:   exam.ID,
		}
		if err := tx.Create(&listeningSection).Error; err != nil {
			return fmt.Errorf("create listening section for exam %s: %w", test.TestSetID, err)
		}
		if err := tx.Create(&readingSection).Error; err != nil {
			return fmt.Errorf("create reading section for exam %s: %w", test.TestSetID, err)
		}

		sectionByCode := map[string]uint{
			string(model.SectionCodeListening): listeningSection.ID,
			string(model.SectionCodeReading):   readingSection.ID,
		}
		sectionOrderCounters := map[uint]int{
			listeningSection.ID: 0,
			readingSection.ID:   0,
		}
		groupIDs := make(map[string]uint)
		options := make([]model.Option, 0, 1000)

		for _, row := range questionsByTest[test.TestSetID] {
			sectionID, ok := sectionByCode[row.Section]
			if !ok {
				return fmt.Errorf("toeic question %s has unsupported section %q", row.QuestionID, row.Section)
			}

			var groupID *uint
			if strings.TrimSpace(row.GroupID) != "" {
				groupKey := test.TestSetID + ":" + row.GroupID
				if existingID, ok := groupIDs[groupKey]; ok {
					groupID = &existingID
				} else {
					audioURL := toPublicURLPointer(uploads, row.AudioURL)
					imageURL := toPublicURLPointer(uploads, row.ImageURL)
					group := model.Group{
						Title:       buildGroupTitle(row.Part, row.GroupOrder),
						Instruction: row.PassageText,
						AudioURL:    audioURL,
						ImageURL:    imageURL,
						Transcript:  row.AudioTranscript,
						Explanation: row.Explanation,
						Order:       row.GroupOrder,
						SectionID:   sectionID,
					}
					if err := tx.Create(&group).Error; err != nil {
						return fmt.Errorf("create toeic group %s: %w", groupKey, err)
					}
					groupIDs[groupKey] = group.ID
					groupID = &group.ID
				}
			}

			partCode, err := parsePartCode(row.Part)
			if err != nil {
				return fmt.Errorf("toeic question %s: %w", row.QuestionID, err)
			}

			sectionOrderCounters[sectionID]++
			question := model.Question{
				Content:     row.QuestionText,
				Part:        &partCode,
				Explanation: row.Explanation,
				Order:       sectionOrderCounters[sectionID],
				SectionID:   sectionID,
				GroupID:     groupID,
			}
			if err := tx.Create(&question).Error; err != nil {
				return fmt.Errorf("create toeic question %s: %w", row.QuestionID, err)
			}

			answers := answersByQuestion[row.QuestionID]
			if len(answers) == 0 {
				return fmt.Errorf("toeic question %s has no answers", row.QuestionID)
			}

			fallbackOptions := parseTranscriptOptions(row.AudioTranscript)
			for _, answer := range answers {
				key, err := optionKeyFromOrder(answer.AnswerOrder)
				if err != nil {
					return fmt.Errorf("toeic question %s: %w", row.QuestionID, err)
				}

				content := cleanText(answer.Content)
				if content == "" {
					content = fallbackOptions[key]
				}

				options = append(options, model.Option{
					Key:        key,
					Content:    nullableStringPointer(content),
					IsCorrect:  answer.IsCorrect || answer.AnswerOrder == row.CorrectAnswerOrder,
					Order:      answer.AnswerOrder,
					QuestionID: question.ID,
				})
			}
		}

		if len(options) > 0 {
			if err := tx.CreateInBatches(options, 1000).Error; err != nil {
				return fmt.Errorf("create toeic options for exam %s: %w", test.TestSetID, err)
			}
		}
	}

	return nil
}

func resetExams(tx *gorm.DB) error {
	models := []any{
		&model.Option{},
		&model.Question{},
		&model.Group{},
		&model.Section{},
		&model.Exam{},
	}

	for _, m := range models {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(m).Error; err != nil {
			return fmt.Errorf("reset exams data: %w", err)
		}
	}

	return nil
}

func buildGroupTitle(part string, groupOrder int) string {
	part = strings.TrimSpace(strings.TrimPrefix(part, "PART_"))
	if part == "" {
		return fmt.Sprintf("Group %d", groupOrder)
	}
	return fmt.Sprintf("Part %s - Group %d", part, groupOrder)
}

func parseTranscriptOptions(transcript string) map[model.OptionKey]string {
	result := make(map[model.OptionKey]string)
	matches := transcriptOptionExp.FindAllStringSubmatchIndex(transcript, -1)
	for i, match := range matches {
		if len(match) < 4 {
			continue
		}

		key := transcript[match[2]:match[3]]
		contentStart := match[1]
		contentEnd := len(transcript)
		if i+1 < len(matches) {
			contentEnd = matches[i+1][0]
		}

		optionKey, err := parseOptionKey(key)
		if err != nil {
			continue
		}

		result[optionKey] = cleanText(transcript[contentStart:contentEnd])
	}
	return result
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

func extractYear(value string) *int {
	match := yearPattern.FindString(value)
	if match == "" {
		return nil
	}
	year, err := strconv.Atoi(match)
	if err != nil {
		return nil
	}
	return &year
}

func parsePartCode(value string) (model.PartCode, error) {
	switch strings.TrimSpace(strings.ToUpper(value)) {
	case "PART_1":
		return model.Part1, nil
	case "PART_2":
		return model.Part2, nil
	case "PART_3":
		return model.Part3, nil
	case "PART_4":
		return model.Part4, nil
	case "PART_5":
		return model.Part5, nil
	case "PART_6":
		return model.Part6, nil
	case "PART_7":
		return model.Part7, nil
	default:
		return "", fmt.Errorf("unsupported part %q", value)
	}
}

func parseOptionKey(value string) (model.OptionKey, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "A":
		return model.OptionA, nil
	case "B":
		return model.OptionB, nil
	case "C":
		return model.OptionC, nil
	case "D":
		return model.OptionD, nil
	default:
		return "", fmt.Errorf("unsupported option key %q", value)
	}
}

func optionKeyFromOrder(order int) (model.OptionKey, error) {
	switch order {
	case 1:
		return model.OptionA, nil
	case 2:
		return model.OptionB, nil
	case 3:
		return model.OptionC, nil
	case 4:
		return model.OptionD, nil
	default:
		return "", fmt.Errorf("unsupported option order %d", order)
	}
}

func optionOrderFromKey(value string) int {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "A":
		return 1
	case "B":
		return 2
	case "C":
		return 3
	case "D":
		return 4
	default:
		return 99
	}
}

func normalizeOptionContent(value string, text string) string {
	text = cleanText(text)
	value = strings.ToUpper(strings.TrimSpace(value))
	if text == "" {
		return value
	}

	prefixes := []string{
		value + ".",
		value + ")",
		value + ":",
		value + " ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToUpper(text), prefix) {
			return cleanText(strings.TrimSpace(text[len(prefix):]))
		}
	}

	return text
}

func nullableStringPointer(value string) *string {
	value = cleanText(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringPointer(value string) *string {
	return &value
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

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing media file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("media path is a directory: %s", path)
	}
	if info.Size() <= 0 {
		return fmt.Errorf("media file is empty: %s", path)
	}
	return nil
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimSpace(value)
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

func parseUint(value, name string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return uint(parsed), nil
}

func parseInt(value, name string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", name, value, err)
	}
	return parsed, nil
}

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode >= http.StatusInternalServerError
}
