package config

import "os"

type AppConfig struct {
	Port             string
	Mode             string
	GRPCPort         string
	UsersServiceAddr string
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port:             getEnv("APP_PORT", "8080"),
		Mode:             getEnv("APP_MODE", "development"),
		GRPCPort:         getEnv("GRPC_PORT", "50053"),
		UsersServiceAddr: getEnv("USERS_SERVICE_ADDR", "users:50052"),
	}

	return config, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
