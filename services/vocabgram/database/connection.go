package database

import (
	"errors"
	"log"

	"ego/services/vocabgram/config"
	"ego/services/vocabgram/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	log.Println("[DATABASE] VocabGram database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("[DATABASE] Running VocabGram database migrations...")

	err := db.AutoMigrate(
		// Dictionary models
		&model.Word{},
		&model.Phonetic{},
		&model.Meaning{},
		&model.Definition{},
		&model.MeaningSynonym{},
		&model.MeaningAntonym{},
		&model.DefinitionSynonym{},
		&model.DefinitionAntonym{},
		&model.UserFlashcard{},
		&model.Topic{},
		&model.TopicWord{},

		// Grammar models
		&model.GrammarUnit{},
		&model.GrammarExample{},
		&model.GrammarPractice{},
		&model.UserUnitProgress{},
	)
	if err != nil {
		return err
	}

	log.Println("[DATABASE] VocabGram database migrations completed")
	return nil
}

