package config

import (
	"os"
	"strconv"
	"time"

	"ego/platform/logger"
)

type AppConfig struct {
	Port     string
	Mode     string
	GRPCPort string

	IdentityServiceAddr string

	R2Region         string
	R2Endpoint       string
	R2Host           string
	R2PublicEndpoint string
	R2AccessKey      string
	R2SecretKey      string
	R2Bucket         string
	R2PublicRead     bool
	R2Expiry         time.Duration
}

func LoadAppConfig() *AppConfig {
	config := &AppConfig{
		Port:                mustEnv("APP_PORT"),
		Mode:                mustEnv("APP_MODE"),
		GRPCPort:            mustEnv("GRPC_PORT"),
		IdentityServiceAddr: mustEnv("IDENTITY_SERVICE_ADDR"),
		R2Region:            mustEnv("R2_REGION"),
		R2Endpoint:          mustEnv("R2_ENDPOINT"),
		R2Host:              mustEnv("R2_HOST"),
		R2PublicEndpoint:    mustEnv("R2_PUBLIC_ENDPOINT"),
		R2AccessKey:         mustEnv("R2_ACCESS_KEY"),
		R2SecretKey:         mustEnv("R2_SECRET_KEY"),
		R2Bucket:            mustEnv("R2_BUCKET"),
		R2PublicRead:        parseBool("R2_PUBLIC_READ"),
		R2Expiry:            parseDuration("R2_EXPIRY"),
	}

	return config
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

func parseBool(key string) bool {
	value := mustEnv(key)
	b, err := strconv.ParseBool(value)
	if err != nil {
		logger.Log.Fatal().Err(err).Str("key", key).Str("value", value).Msg("[CONFIG] Invalid boolean format")
	}
	return b
}
