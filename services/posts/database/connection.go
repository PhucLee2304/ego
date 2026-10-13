package database

import (
	"errors"
	"log"

	"ego/services/posts/config"
	"ego/services/posts/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(config *config.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(config.DSN), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, err
	}
	log.Println("Database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("Running database migrations...")
	if err := db.AutoMigrate(
		&model.Post{},
		&model.PostComment{},
		&model.PostLike{},
		&model.CommentLike{},
		&model.PostModerationAudit{},
		&model.NotificationOutbox{},
	); err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}
