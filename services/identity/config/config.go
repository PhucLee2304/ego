package config

import (
	"fmt"
	"os"
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

	StorageServiceAddr string
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:     getEnv("APP_PORT", "8080"),
		Mode:     getEnv("APP_MODE", "development"),
		GRPCPort: getEnv("GRPC_PORT", "50052"),

		DBHost:     getEnv("DB_HOST", "users-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "23042004"),
		DBName:     getEnv("DB_NAME", "usersdb"),

		StorageServiceAddr: getEnv("STORAGE_SERVICE_ADDR", "storage:50054"),
	}

	config.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	return config, nil
}

type JwtConfig struct {
	JwtSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

func LoadJwtConfig() (*JwtConfig, error) {
	accessTokenExpiry, err := time.ParseDuration(os.Getenv("ACCESS_TOKEN_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("[ERROR][Config][JWT] Failed to parse ACCESS_TOKEN_EXPIRY: %v", err)
	}

	refreshTokenExpiry, err := time.ParseDuration(os.Getenv("REFRESH_TOKEN_EXPIRY"))
	if err != nil {
		return nil, fmt.Errorf("[ERROR][Config][JWT] Failed to parse REFRESH_TOKEN_EXPIRY: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("[ERROR][Config][JWT] JWT_SECRET is not set")
	}

	cfg := JwtConfig{
		JwtSecret:          jwtSecret,
		AccessTokenExpiry:  accessTokenExpiry,
		RefreshTokenExpiry: refreshTokenExpiry,
	}

	return &cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
