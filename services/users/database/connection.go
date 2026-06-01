package database

import (
	"ego/services/users/config"
	"ego/services/users/internal/model"
	"errors"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	log.Println("Database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("database connection is nil")
	}

	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		&model.User{},
	)
	if err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}
