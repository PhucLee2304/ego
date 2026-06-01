package config

import (
	"fmt"
	"os"
	"time"
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

type FirebaseConfig struct {
	FirebaseCredentialsPath string
}

func LoadFirebaseConfig() (*FirebaseConfig, error) {
	cfg := FirebaseConfig{
		FirebaseCredentialsPath: os.Getenv("FIREBASE_CREDENTIALS_PATH"),
	}
	return &cfg, nil
}
