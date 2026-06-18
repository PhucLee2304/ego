package config

import (
	"os"
	"time"
)

type AppConfig struct {
	Port     string
	Mode     string
	GRPCPort string

	AuthServiceAddr string

	R2Region    string
	R2Endpoint  string
	R2Host      string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
	R2Expiry    time.Duration
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:            getEnv("APP_PORT", "8082"),
		Mode:            getEnv("APP_MODE", "development"),
		GRPCPort:        getEnv("GRPC_PORT", "50054"),
		AuthServiceAddr: getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
		R2Region:        getEnv("R2_REGION", "auto"),
		R2Endpoint:      getEnv("R2_ENDPOINT", "http://localhost:9002"),
		R2Host:          getEnv("R2_HOST", "localhost:9002"),
		R2AccessKey:     getEnv("R2_ACCESS_KEY", "root"),
		R2SecretKey:     getEnv("R2_SECRET_KEY", "root"),
		R2Bucket:        getEnv("R2_BUCKET", "ego"),
		R2Expiry:        parseDuration("R2_EXPIRY", "15m"),
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func parseDuration(key, fallback string) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	if d, err := time.ParseDuration(fallback); err == nil {
		return d
	}
	return 0
}
