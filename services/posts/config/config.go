package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"ego/platform/logger"
)

type AppConfig struct {
	Port string
	Mode string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DSN        string

	IdentityServiceAddr      string
	LearningServiceAddr      string
	NotificationsServiceAddr string
	StorageServiceAddr       string

	ImageMaxBytes   int64
	FileMaxBytes    int64
	VideoMaxBytes   int64
	PostMaxBytes    int64
	MaxAttachments  int
	UploadIntentTTL time.Duration
}

func LoadAppConfig() *AppConfig {
	config := &AppConfig{
		Port: mustEnv("APP_PORT"),
		Mode: mustEnv("APP_MODE"),

		DBHost:     mustEnv("DB_HOST"),
		DBPort:     mustEnv("DB_PORT"),
		DBUser:     mustEnv("DB_USER"),
		DBPassword: mustEnv("DB_PASSWORD"),
		DBName:     mustEnv("DB_NAME"),

		IdentityServiceAddr:      mustEnv("IDENTITY_SERVICE_ADDR"),
		LearningServiceAddr:      mustEnv("LEARNING_SERVICE_ADDR"),
		NotificationsServiceAddr: mustEnv("NOTIFICATIONS_SERVICE_ADDR"),
		StorageServiceAddr:       mustEnv("STORAGE_SERVICE_ADDR"),
		ImageMaxBytes:            parseInt64("POST_IMAGE_MAX_BYTES"),
		FileMaxBytes:             parseInt64("POST_FILE_MAX_BYTES"),
		VideoMaxBytes:            parseInt64("POST_VIDEO_MAX_BYTES"),
		PostMaxBytes:             parseInt64("POST_TOTAL_MAX_BYTES"),
		MaxAttachments:           parseInt("POST_MAX_ATTACHMENTS"),
		UploadIntentTTL:          time.Duration(parseInt64("POST_UPLOAD_TTL_MINUTES")) * time.Minute,
	}

	config.DSN = fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBName,
	)

	return config
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		logger.Log.Fatal().Str("key", key).Msg("[CONFIG] Required environment variable is not set")
	}
	return value
}

func parseInt(key string) int {
	value := mustEnv(key)
	n, err := strconv.Atoi(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid integer format")
	}
	return n
}

func parseInt64(key string) int64 {
	value := mustEnv(key)
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid integer format")
	}
	return n
}
