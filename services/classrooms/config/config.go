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

	AuthServiceAddr          string
	UsersServiceAddr         string
	ExamsServiceAddr         string
	NotificationsServiceAddr string
	LiveKitURL               string
	LiveKitPublicURL         string
	LiveKitAPIKey            string
	LiveKitAPISecret         string

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

		AuthServiceAddr:          getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
		UsersServiceAddr:         getEnv("USERS_SERVICE_ADDR", "users:50052"),
		ExamsServiceAddr:         getEnv("EXAMS_SERVICE_ADDR", "exams:50056"),
		NotificationsServiceAddr: getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications:50058"),
		LiveKitURL:               getEnv("LIVEKIT_URL", "http://livekit:7880"),
		LiveKitPublicURL:         getEnv("LIVEKIT_PUBLIC_URL", "ws://localhost:7880"),
		LiveKitAPIKey:            getEnv("LIVEKIT_API_KEY", "devkey"),
		LiveKitAPISecret:         getEnv("LIVEKIT_API_SECRET", "local-development-secret-key-000001"),

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
