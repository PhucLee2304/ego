package config

import (
	"fmt"
	"os"

	"ego/platform/logger"
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
		Port:                    mustEnv("APP_PORT"),
		GRPCPort:                mustEnv("GRPC_PORT"),
		DBHost:                  mustEnv("DB_HOST"),
		DBPort:                  mustEnv("DB_PORT"),
		DBUser:                  mustEnv("DB_USER"),
		DBPassword:              mustEnv("DB_PASSWORD"),
		DBName:                  mustEnv("DB_NAME"),
		IdentityServiceAddr:     mustEnv("IDENTITY_SERVICE_ADDR"),
		FirebaseCredentialsPath: mustEnv("FIREBASE_CREDENTIALS_PATH"),
	}
	cfg.DSN = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	return cfg
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		logger.Log.Fatal().Str("key", key).Msg("[CONFIG] Required environment variable is not set")
	}
	return value
}
