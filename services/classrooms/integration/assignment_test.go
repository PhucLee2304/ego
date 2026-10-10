package integration_test

import (
	"os"
	"testing"
	"time"

	"ego/services/classrooms/database"
	"ego/services/classrooms/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAssignmentScheduleAndExamUniqueness(t *testing.T) {
	dsn := os.Getenv("CLASSROOMS_TEST_DSN")
	if dsn == "" {
		t.Skip("CLASSROOMS_TEST_DSN is not configured")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	classroom := &model.Classroom{Code: "TEST0001", Name: "Test class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	opensAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	assignment := &model.Assignment{ClassroomID: classroom.ID, ExamID: 99, Title: "Assignment",
		Duration: 45, OpensAt: opensAt, DueAt: opensAt.Add(2 * time.Hour), Active: true}
	if err := db.Create(assignment).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := *assignment
	duplicate.Model = gorm.Model{}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate classroom/exam assignment to be rejected")
	}
}
