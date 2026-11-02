package config

import (
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

	IdentityServiceAddr    string
	SeedDictionaryFilePath string
	SeedGrammarFilePath    string
	SeedTopicsFilePath     string
	AudioBasePath          string
	AudioBaseURL           string
	SeedBatchSize          int
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8080"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50056"),

		DBHost:     getEnv("DB_HOST", "vocabgram-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "23042004"),
		DBName:     getEnv("DB_NAME", "vocabgramdb"),

		IdentityServiceAddr:    getEnv("IDENTITY_SERVICE_ADDR", "identity:50052"),
		SeedDictionaryFilePath: getEnv("SEED_DICTIONARY_FILE_PATH", "/app/services/vocabgram/cmd/seed/dictionary_data_offline.json"),
		SeedGrammarFilePath:    getEnv("SEED_GRAMMAR_FILE_PATH", "/app/services/vocabgram/cmd/seed/Grammar.json"),
		SeedTopicsFilePath:     getEnv("SEED_TOPICS_FILE_PATH", "/app/services/vocabgram/cmd/seed/topics.json"),
		AudioBasePath:          getEnv("AUDIO_BASE_PATH", "/app/assets"),
		AudioBaseURL:           getEnv("AUDIO_BASE_URL", "/dictionary/assets"),
		SeedBatchSize:          getIntEnv("SEED_BATCH_SIZE", 500, 2000),
	}

	config.DSN = fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName,
	)

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

