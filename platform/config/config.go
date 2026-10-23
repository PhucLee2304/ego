package config

import (
	"fmt"
	"os"
)

type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Name     string
	Conn     string
}

func LoadDBConfig() (*DBConfig, error) {
	cfg := DBConfig{
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		Name:     os.Getenv("DB_NAME"),
	}

	cfg.Conn = fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name)

	return &cfg, nil
}

type FirebaseConfig struct {
	FirebaseCredentialsPath string
}

func LoadFirebaseConfig() (*FirebaseConfig, error) {
	cfg := FirebaseConfig{
		FirebaseCredentialsPath: os.Getenv("FIREBASE_CREDENTIALS_PATH"),
	}
	return &cfg, nil
}
