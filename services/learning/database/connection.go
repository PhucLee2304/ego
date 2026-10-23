package database

import (
	"ego/platform/logger"
	"ego/services/learning/config"
	"ego/services/learning/internal/model"
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

// Migrate chạy AutoMigrate cho mọi model rồi raw SQL trong model/index.go.
func Migrate(db *gorm.DB) error {
	if db == nil {
		return errors.New("[DATABASE] Database connection is nil")
	}

	logger.Log.Info().Msg("[STARTUP] Running database migrations")

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
		&model.Classroom{},
		&model.Member{},
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

	logger.Log.Info().Msg("[STARTUP] Database migrations completed")
	return nil
}
