package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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

	IdentityServiceAddr string
	StorageServiceAddr  string

	SeedDir           string
	SeedUploadWorkers int
	SeedUploadRetries int
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8080"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50055"),

		DBHost:     getEnv("DB_HOST", "knowledge-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "knowledgedb"),

		IdentityServiceAddr: getEnv("IDENTITY_SERVICE_ADDR", "identity:50052"),
		StorageServiceAddr:  getEnv("STORAGE_SERVICE_ADDR", "storage:50054"),

		SeedDir:           getEnv("SEED_DIR", "/app/services/knowledge/seed/topics"),
		SeedUploadWorkers: getIntEnv("SEED_UPLOAD_WORKERS", 8, 16),
		SeedUploadRetries: getIntEnv("SEED_UPLOAD_RETRIES", 5, 10),
	}

	if config.DBPassword == "" {
		return nil, errors.New("[CONFIG] DB_PASSWORD is not set")
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
