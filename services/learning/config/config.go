package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"ego/platform/logger"
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

	// Giám sát làm bài (PLAN_ATTEMPT_INTEGRITY.md mục 1.3–1.7).
	IntegrityTakeoverDelayMs int
	IntegrityConsentVersion  string

	SeedDir           string
	SeedUploadWorkers int
	SeedUploadRetries int
}

// DefaultIntegrityConsentVersion phải khớp hằng integrityConsentVersion của FE.
const DefaultIntegrityConsentVersion = "2026-11-v4"

func LoadAppConfig() *AppConfig {
	config := &AppConfig{
		Port:     mustEnv("APP_PORT"),
		Mode:     mustEnv("APP_MODE"),
		GRPCPort: mustEnv("GRPC_PORT"),

		DBHost:     mustEnv("DB_HOST"),
		DBPort:     mustEnv("DB_PORT"),
		DBUser:     mustEnv("DB_USER"),
		DBPassword: mustEnv("DB_PASSWORD"),
		DBName:     mustEnv("DB_NAME"),

		IdentityServiceAddr:      mustEnv("IDENTITY_SERVICE_ADDR"),
		StorageServiceAddr:       mustEnv("STORAGE_SERVICE_ADDR"),
		NotificationsServiceAddr: mustEnv("NOTIFICATIONS_SERVICE_ADDR"),

		LiveKitURL:          mustEnv("LIVEKIT_URL"),
		LiveKitPublicURL:    mustEnv("LIVEKIT_PUBLIC_URL"),
		LiveKitAPIKey:       mustEnv("LIVEKIT_API_KEY"),
		LiveKitAPISecret:    mustEnv("LIVEKIT_API_SECRET"),
		LiveKitJoinTokenTTL: parseDuration("LIVEKIT_JOIN_TOKEN_TTL"),

		MaxStudentsPerClass: parseInt("MAX_STUDENTS_PER_CLASS"),

		TOEICStrictFlowEnabled: parseBool("TOEIC_STRICT_FLOW_ENABLED"),
		TOEICListeningGapMs: [4]int{
			parseInt("TOEIC_LISTENING_GAP_PART1_MS"),
			parseInt("TOEIC_LISTENING_GAP_PART2_MS"),
			parseInt("TOEIC_LISTENING_GAP_PART3_MS"),
			parseInt("TOEIC_LISTENING_GAP_PART4_MS"),
		},
		TOEICListeningGraceMs: parseInt("TOEIC_LISTENING_GRACE_MS"),

		IntegrityTakeoverDelayMs: parseInt("INTEGRITY_TAKEOVER_DELAY_MS"),
		IntegrityConsentVersion:  mustEnv("INTEGRITY_CONSENT_VERSION"),

		SeedDir:           mustEnv("SEED_DIR"),
		SeedUploadWorkers: parseInt("SEED_UPLOAD_WORKERS"),
		SeedUploadRetries: parseInt("SEED_UPLOAD_RETRIES"),
	}

	config.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	return config
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		logger.Log.Fatal().Str("key", key).Msg("[CONFIG] Required environment variable is not set")
	}
	return value
}

func parseDuration(key string) time.Duration {
	value := mustEnv(key)
	d, err := time.ParseDuration(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid duration format")
	}
	return d
}

func parseInt(key string) int {
	value := mustEnv(key)
	n, err := strconv.Atoi(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid integer format")
	}
	return n
}

func parseBool(key string) bool {
	value := mustEnv(key)
	b, err := strconv.ParseBool(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid boolean format")
	}
	return b
}
