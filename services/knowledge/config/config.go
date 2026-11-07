package config

import (
	"fmt"
	"os"
	"strconv"

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

	IdentityServiceAddr string
	StorageServiceAddr  string

	SeedDir           string
	SeedUploadWorkers int
	SeedUploadRetries int
}

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

		IdentityServiceAddr: mustEnv("IDENTITY_SERVICE_ADDR"),
		StorageServiceAddr:  mustEnv("STORAGE_SERVICE_ADDR"),

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

func parseInt(key string) int {
	value := mustEnv(key)
	n, err := strconv.Atoi(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid integer format")
	}
	return n
}
