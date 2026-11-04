package database

import (
	"ego/platform/logger"
	"ego/services/knowledge/config"
	"ego/services/knowledge/internal/model"
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(cfg *config.AppConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	logger.Log.Info().Msg("[STARTUP] Database connection established")
	return db, nil
}

// Migrate chạy AutoMigrate cho mọi model của knowledge. Model của từng domain
// (topic, dictionary, grammar) được thêm vào đây khi domain đó được đưa vào.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	logger.Log.Info().Msg("[STARTUP] Running database migrations")

	err := db.AutoMigrate(
		// Topic
		&model.Topic{},
		&model.Section{},
		&model.Lesson{},
		&model.Transcript{},
	)
	if err != nil {
		return err
	}

	logger.Log.Info().Msg("[STARTUP] Database migrations completed")
	return nil
}
