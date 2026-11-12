package database

import (
	"errors"

	"ego/platform/logger"
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
	logger.Log.Info().Msg("[STARTUP] Database connection established")
	return db, nil
}

func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	logger.Log.Info().Msg("[STARTUP] Running database migrations")
	if err := db.AutoMigrate(
		&model.Post{},
		&model.PostComment{},
		&model.PostLike{},
		&model.CommentLike{},
		&model.PostModerationAudit{},
		&model.NotificationOutbox{},
		&model.PostUploadIntent{},
		&model.PostAttachment{},
	); err != nil {
		return err
	}

	logger.Log.Info().Msg("[STARTUP] Database migrations completed")
	return nil
}
