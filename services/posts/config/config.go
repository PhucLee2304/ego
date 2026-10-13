package config

import (
	"fmt"
	"os"
)

type AppConfig struct {
	Port string
	Mode string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DSN        string

	AuthServiceAddr          string
	UsersServiceAddr         string
	ClassroomsServiceAddr    string
	NotificationsServiceAddr string
}

func LoadAppConfig() (*AppConfig, error) {
	config := &AppConfig{
		Port: getEnv("APP_PORT", "8080"),
		Mode: getEnv("APP_MODE", "development"),

		DBHost:     getEnv("DB_HOST", "posts-db"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "postsdb"),

		AuthServiceAddr:          getEnv("AUTH_SERVICE_ADDR", "auth:50053"),
		UsersServiceAddr:         getEnv("USERS_SERVICE_ADDR", "users:50052"),
		ClassroomsServiceAddr:    getEnv("CLASSROOMS_SERVICE_ADDR", "classrooms:50057"),
		NotificationsServiceAddr: getEnv("NOTIFICATIONS_SERVICE_ADDR", "notifications:50058"),
	}

	config.DSN = fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost,
		config.DBPort,
		config.DBUser,
		config.DBPassword,
		config.DBName,
	)

	return config, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
