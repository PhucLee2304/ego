package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type AppConfig struct {
	Port     string
	Mode     string
	GRPCPort string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DSN        string

	IdentityServiceAddr      string
	StorageServiceAddr       string
	NotificationsServiceAddr string

	LiveKitURL          string
	LiveKitPublicURL    string
	LiveKitAPIKey       string
	LiveKitAPISecret    string
	LiveKitJoinTokenTTL time.Duration

	MaxStudentsPerClass int

	// Luồng TOEIC strict (PLAN_TOEIC_ATTEMPT.md T2). Tắt thì attempt TOEIC tạo ra với Flow = FREE.
	TOEICStrictFlowEnabled bool
	// Thời gian trả lời mục tiêu sau audio của mỗi trang nghe, theo part 1..4 (ms).
	TOEICListeningGapMs   [4]int
	TOEICListeningGraceMs int

	SeedDir           string
	SeedUploadWorkers int
	SeedUploadRetries int
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8080"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50059"),

		DBHost:     getEnv("DB_HOST", "learning-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "learningdb"),

		IdentityServiceAddr:      getEnv("IDENTITY_SERVICE_ADDR", "identity:50052"),
		StorageServiceAddr:       getEnv("STORAGE_SERVICE_ADDR", "storage:50054"),
		NotificationsServiceAddr: getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications:50058"),

		LiveKitURL:          getEnv("LIVEKIT_URL", "http://livekit:7880"),
		LiveKitPublicURL:    getEnv("LIVEKIT_PUBLIC_URL", "ws://localhost:7880"),
		LiveKitAPIKey:       getEnv("LIVEKIT_API_KEY", ""),
		LiveKitAPISecret:    getEnv("LIVEKIT_API_SECRET", ""),
		LiveKitJoinTokenTTL: getDurationEnv("LIVEKIT_JOIN_TOKEN_TTL", 5*time.Minute),

		MaxStudentsPerClass: getIntEnv("MAX_STUDENTS_PER_CLASS", 50, 50),

		TOEICStrictFlowEnabled: getBoolEnv("TOEIC_STRICT_FLOW_ENABLED", false),
		TOEICListeningGapMs: [4]int{
			getIntEnv("TOEIC_LISTENING_GAP_PART1_MS", 5000, 60000),
			getIntEnv("TOEIC_LISTENING_GAP_PART2_MS", 5000, 60000),
			getIntEnv("TOEIC_LISTENING_GAP_PART3_MS", 8000, 60000),
			getIntEnv("TOEIC_LISTENING_GAP_PART4_MS", 8000, 60000),
		},
		TOEICListeningGraceMs: getIntEnv("TOEIC_LISTENING_GRACE_MS", 20000, 120000),

		SeedDir:           getEnv("SEED_DIR", "services/learning/seed"),
		SeedUploadWorkers: getIntEnv("SEED_UPLOAD_WORKERS", 8, 16),
		SeedUploadRetries: getIntEnv("SEED_UPLOAD_RETRIES", 5, 10),
	}

	if config.LiveKitAPIKey == "" || config.LiveKitAPISecret == "" {
		return nil, errors.New("[CONFIG] LIVEKIT_API_KEY and LIVEKIT_API_SECRET are required")
	}

	config.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	return config, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getIntEnv(key string, fallback, max int) int {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	if parsed > max {
		return max
	}
	return parsed
}

func getBoolEnv(key string, fallback bool) bool {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value, exists := os.LookupEnv(key)
	if !exists {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
