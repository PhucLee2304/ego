// Command seed_topics nạp dữ liệu topic (topic → section → lesson → transcript) từ CSV
// trong SEED_DIR, upload media lên storage (thư mục MinIO "topics") rồi ghi vào DB
// trong một transaction. Chạy lại sẽ xóa dữ liệu topic cũ trong DB trước khi nạp.
package main

import (
	"context"
	"time"

	storageClient "ego/api/gen/go/storage"
	"ego/platform/logger"
	"ego/platform/rpc"
	"ego/services/knowledge/config"
	"ego/services/knowledge/database"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const skippedTopicID uint = 11

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
