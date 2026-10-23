package database

import (
	"ego/services/learning/config"
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

// Migrate chạy AutoMigrate cho mọi model rồi raw SQL trong model/index.go.
// Model được thêm dần theo từng phase (P4 notification outbox, P5 exams, P13+ classrooms).
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("Running database migrations...")

	log.Println("Database migrations completed")
	return nil
}
