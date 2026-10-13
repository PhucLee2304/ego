package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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

	AuthServiceAddr          string
	UsersServiceAddr         string
	ClassroomsServiceAddr    string
	NotificationsServiceAddr string
	StorageServiceAddr       string

	ImageMaxBytes   int64
	FileMaxBytes    int64
	PostMaxBytes    int64
	MaxAttachments  int
	UploadIntentTTL time.Duration
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port: getEnv("APP_PORT", "8080"),
		Mode: getEnv("APP_MODE", "development"),

		DBHost:     getEnv("DB_HOST", "posts-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "postsdb"),

		AuthServiceAddr:          getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
		UsersServiceAddr:         getEnv("USERS_SERVICE_ADDR", "users:50052"),
		ClassroomsServiceAddr:    getEnv("CLASSROOMS_SERVICE_ADDR", "classrooms:50057"),
		NotificationsServiceAddr: getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications:50058"),
		StorageServiceAddr:       getEnv("STORAGE_SERVICE_ADDR", "storage:50055"),
		ImageMaxBytes:            getEnvInt64("POST_IMAGE_MAX_BYTES", 5*1024*1024),
		FileMaxBytes:             getEnvInt64("POST_FILE_MAX_BYTES", 10*1024*1024),
		PostMaxBytes:             getEnvInt64("POST_TOTAL_MAX_BYTES", 50*1024*1024),
		MaxAttachments:           int(getEnvInt64("POST_MAX_ATTACHMENTS", 10)),
		UploadIntentTTL:          time.Duration(getEnvInt64("POST_UPLOAD_TTL_MINUTES", 60)) * time.Minute,
	}

	config.DSN = fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBName,
	)

	return config, nil
}

func getEnvInt64(key string, fallback int64) int64 {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
