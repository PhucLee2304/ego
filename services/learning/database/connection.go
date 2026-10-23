package database

import (
	"ego/services/learning/config"
	"ego/services/learning/internal/model"
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
// Model classrooms được thêm từ P13.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		&model.Exam{},
		&model.Section{},
		&model.Group{},
		&model.Question{},
		&model.Option{},
		&model.Attempt{},
		&model.AttemptAnswer{},
		&model.History{},
		&model.ExamAudit{},
		&model.NotificationOutbox{},
	)
	if err != nil {
		return err
	}

	for _, sql := range []string{
		model.CreateAttemptActiveUserUniqueIndexSQL,
		model.CreateAttemptClassroomAssignmentUniqueIndexSQL,
		model.CreateOptionCorrectQuestionUniqueIndexSQL,
		model.CreateOptionQuestionKeyUniqueIndexSQL,
		model.CreateAssignmentQuestionOptionsConstraintSQL,
	} {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}

	log.Println("Database migrations completed")
	return nil
}
