package config

import (
	"fmt"
	"os"
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

	StorageServiceAddr       string
	NotificationsServiceAddr string
	LearningServiceAddr      string

	FirebaseCredentialsPath string
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

		StorageServiceAddr:       mustEnv("STORAGE_SERVICE_ADDR"),
		NotificationsServiceAddr: mustEnv("NOTIFICATIONS_SERVICE_ADDR"),
		LearningServiceAddr:      mustEnv("LEARNING_SERVICE_ADDR"),

		FirebaseCredentialsPath: mustEnv("FIREBASE_CREDENTIALS_PATH"),
	}

	config.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	return config
}

type JwtConfig struct {
	JwtSecret          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

func LoadJwtConfig() *JwtConfig {
	return &JwtConfig{
		JwtSecret:          mustEnv("JWT_SECRET"),
		AccessTokenExpiry:  parseDuration("ACCESS_TOKEN_EXPIRY"),
		RefreshTokenExpiry: parseDuration("REFRESH_TOKEN_EXPIRY"),
	}
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
