package config

import (
	"os"

	"ego/platform/logger"
)

type AppConfig struct {
	Port                string
	Mode                string
	IdentityServiceAddr string
	GeminiAPIKey        string
	GeminiModel         string
}

func LoadAppConfig() *AppConfig {
	model := mustEnv("GEMINI_MODEL")
	if model == "" || model == "gemini-2.5-flash" || model == "gemini-2.5-flash-lite" {
		model = "gemini-flash-lite-latest"
	}
	return &AppConfig{
		Port:                mustEnv("APP_PORT"),
		Mode:                mustEnv("APP_MODE"),
		IdentityServiceAddr: mustEnv("IDENTITY_SERVICE_ADDR"),
		GeminiAPIKey:        mustEnv("GEMINI_API_KEY"),
		GeminiModel:         model,
	}
}

func mustEnv(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		logger.Log.Fatal().Str("key", key).Msg("[CONFIG] Required environment variable is not set")
	}
	return value
}
