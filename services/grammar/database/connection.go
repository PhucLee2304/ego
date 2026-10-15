package database

import (
	"ego/services/grammar/config"
	"ego/services/grammar/internal/model"
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
	log.Println("[DATABASE] Grammar database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("[DATABASE] Running grammar database migrations...")

	err := db.AutoMigrate(
		&model.GrammarUnit{},
		&model.GrammarExample{},
		&model.GrammarPractice{},
		&model.UserUnitProgress{},
	)
	if err != nil {
		return err
	}

	log.Println("[DATABASE] Grammar database migrations completed")
	return nil
}
