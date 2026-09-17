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

	AuthServiceAddr string
	ExamsServiceURL string

	MaxStudentsPerClass int
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8080"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50057"),

		DBHost:     getEnv("DB_HOST", "classrooms-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "classroomsdb"),

		AuthServiceAddr: getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
		ExamsServiceURL: getEnv("EXAMS_SERVICE_URL", "exams:50056"),

		MaxStudentsPerClass: getIntEnv("MAX_STUDENTS_PER_CLASS", 50, 50),
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
