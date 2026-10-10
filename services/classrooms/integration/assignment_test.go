package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"ego/services/classrooms/config"
	"ego/services/classrooms/database"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"ego/services/classrooms/internal/service"

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

	repo := repository.NewRepository(db)
	svc := service.New(repo, &config.AppConfig{}, nil, nil)
	ctx := context.Background()
	endsAt := opensAt.Add(time.Hour)
	created, err := svc.CreateSchedule(ctx, classroom.TeacherID, classroom.ID, dto.CreateScheduleRequest{Title: "Live class", StartsAt: opensAt, EndsAt: endsAt})
	if err != nil || created.ID == 0 {
		t.Fatalf("create classroom event: response=%#v err=%v", created, err)
	}
	calendar, err := svc.GetCalendar(ctx, classroom.TeacherID, classroom.ID, dto.CalendarQuery{From: opensAt.Add(-time.Hour), To: assignment.DueAt.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if len(calendar) != 2 || calendar[0].CalendarType != dto.CalendarTypeClassSchedule || calendar[1].CalendarType != dto.CalendarTypeAssignmentDeadline || calendar[1].RelatedAssignmentID == nil || *calendar[1].RelatedAssignmentID != assignment.ID {
		t.Fatalf("expected stored schedule and dynamic assignment deadline, got %#v", calendar)
	}
	updated, err := svc.UpdateSchedule(ctx, classroom.TeacherID, created.ID, dto.UpdateScheduleRequest{Title: "Consultation", StartsAt: opensAt, EndsAt: endsAt})
	if err != nil || updated.Title != "Consultation" || updated.CalendarType != dto.CalendarTypeClassSchedule {
		t.Fatalf("update classroom schedule: response=%#v err=%v", updated, err)
	}
	if err := svc.DeleteSchedule(ctx, classroom.TeacherID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetScheduleByID(ctx, created.ID); err == nil {
		t.Fatal("expected deleted schedule to be hidden by GORM soft delete")
	}
}
