package database

import (
	"ego/services/dictionary/config"
	"ego/services/dictionary/internal/model"
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
	log.Println("[DATABASE] Dictionary database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("[DATABASE] Running dictionary database migrations...")

	err := db.AutoMigrate(
		&model.Word{},
		&model.Phonetic{},
		&model.Meaning{},
		&model.Definition{},
		&model.MeaningSynonym{},
		&model.MeaningAntonym{},
		&model.DefinitionSynonym{},
		&model.DefinitionAntonym{},
		&model.UserFlashcard{},
	)
	if err != nil {
		return err
	}

	log.Println("[DATABASE] Dictionary database migrations completed")
	return nil
}
