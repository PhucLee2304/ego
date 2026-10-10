package database

import (
	"errors"

	"ego/services/notifications/config"
	"ego/services/notifications/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.AppConfig) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{TranslateError: true})
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}
	return db.AutoMigrate(&model.Notification{}, &model.DeviceToken{})
}
