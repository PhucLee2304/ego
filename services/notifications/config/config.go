package config

import (
	"fmt"
	"os"
)

type AppConfig struct {
	Port                    string
	GRPCPort                string
	DBHost                  string
	DBPort                  string
	DBUser                  string
	DBPassword              string
	DBName                  string
	DSN                     string
	IdentityServiceAddr     string
	FirebaseCredentialsPath string
}

func LoadAppConfig() *AppConfig {
	cfg := &AppConfig{
		Port:                    getEnv("APP_PORT", "8080"),
		GRPCPort:                getEnv("GRPC_PORT", "50058"),
		DBHost:                  getEnv("DB_HOST", "notifications-db"),
		DBPort:                  getEnv("DB_PORT", "5432"),
		DBUser:                  getEnv("DB_USER", "postgres"),
		DBPassword:              getEnv("DB_PASSWORD", "23042004"),
		DBName:                  getEnv("DB_NAME", "notificationsdb"),
		IdentityServiceAddr:     getEnv("IDENTITY_SERVICE_ADDR", "identity:50052"),
		FirebaseCredentialsPath: os.Getenv("FIREBASE_CREDENTIALS_PATH"),
	}
	cfg.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	return cfg
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
