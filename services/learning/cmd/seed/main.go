// Seed đề THPT/TOEIC cho learning (port từ exams/cmd/seed, thân hàm giữ nguyên).
package main

import (
	"context"
	"regexp"
	"time"

	storageClient "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/platform/rpc"
	"ego/services/learning/config"
	"ego/services/learning/database"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	ImageURLs          []string
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

	audio, err := probeAudioFiles(ctx, appConfig.SeedDir, data.manifest, appConfig.SeedUploadWorkers)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to probe audio files")
	}

	uploads, err := uploadMedia(ctx, storage, jobs, appConfig.SeedUploadWorkers, appConfig.SeedUploadRetries)
	if err != nil {
		cleanupExams(ctx, storage, err)
		logger.Log.Fatal().Err(err).Msg("[SEED] Failed to upload exam media")
	}

	if err := importData(db, data, uploads, audio); err != nil {
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
