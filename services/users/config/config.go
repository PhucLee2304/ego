package config

import (
	"fmt"
	"os"
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
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8081"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50052"),

		DBHost:     getEnv("DB_HOST", "users-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "23042004"),
		DBName:     getEnv("DB_NAME", "usersdb"),

		AuthServiceAddr: getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
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
