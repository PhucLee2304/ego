package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	classroomsClient "ego/api/gen/go/classrooms"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/classrooms/config"
	"ego/services/classrooms/database"
	"ego/services/classrooms/internal/dto"
	"ego/services/classrooms/internal/model"
	"ego/services/classrooms/internal/repository"
	"ego/services/classrooms/internal/service"
	classroomsSocket "ego/services/classrooms/internal/socket"
	classroomsRpc "ego/services/classrooms/rpc"

	"google.golang.org/grpc"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fakeNotificationClient struct {
	calls   int
	request *notificationsClient.CreateNotificationsRequest
}

func (f *fakeNotificationClient) CreateNotifications(_ context.Context, req *notificationsClient.CreateNotificationsRequest, _ ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error) {
	f.calls++
	f.request = req
	return &notificationsClient.CreateNotificationsResponse{CreatedCount: uint32(len(req.Recipients))}, nil
}

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
	approved := true
	joinedAt := time.Now()
	if err := db.Create(&model.Member{ClassroomID: classroom.ID, UserID: "student", Approved: &approved, JoinedAt: &joinedAt}).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := *assignment
	duplicate.Model = gorm.Model{}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate classroom/exam assignment to be rejected")
	}

	repo := repository.NewRepository(db)
	ctx := context.Background()
	if err := repo.CreateNotificationOutbox(ctx, &model.NotificationOutbox{EventKey: "assignment:test", Recipients: datatypes.JSONSlice[string]{"student"}, Title: "Assignment", Body: "Body", Data: datatypes.JSONMap{"id": "1", "type": "ASSIGNMENT"}, NextAttemptAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	notificationClient := &fakeNotificationClient{}
	notificationSvc := service.New(repo, &config.AppConfig{}, nil, nil)
	notificationSvc.SetNotificationClient(notificationClient)
	if processed, err := notificationSvc.ProcessNotificationOutbox(ctx, 10); err != nil || processed != 1 || notificationClient.calls != 1 {
		t.Fatalf("process notification outbox: processed=%d calls=%d err=%v", processed, notificationClient.calls, err)
	}
	if processed, err := notificationSvc.ProcessNotificationOutbox(ctx, 10); err != nil || processed != 0 || notificationClient.calls != 1 {
		t.Fatalf("processed event was retried: processed=%d calls=%d err=%v", processed, notificationClient.calls, err)
	}
	var publishedUserID string
	var publishedEvents []any
	rpcServer := classroomsRpc.New(repo, func(userID string, payload any) {
		publishedUserID = userID
		publishedEvents = append(publishedEvents, payload)
	})
	createdSubmission, err := rpcServer.CreateAssignmentSubmission(ctx, &classroomsClient.CreateAssignmentSubmissionRequest{
		AssignmentId: uint64(assignment.ID), StudentId: "student", AttemptId: 101, AllowInactive: true,
	})
	if err != nil || createdSubmission.AttemptId != 101 || publishedUserID != classroom.TeacherID || len(publishedEvents) != 1 {
		t.Fatalf("create submission realtime event: response=%#v teacher=%q events=%d err=%v", createdSubmission, publishedUserID, len(publishedEvents), err)
	}
	createdEvent, ok := publishedEvents[0].(classroomsSocket.GradebookUpdatedEvent)
	if !ok || createdEvent.Payload.Status != string(model.SubmissionStatusInProgress) || createdEvent.Payload.AssignmentID != assignment.ID {
		t.Fatalf("unexpected created gradebook event: %#v", publishedEvents[0])
	}
	updatedSubmission, err := rpcServer.UpdateAssignmentSubmissionResult(ctx, &classroomsClient.UpdateAssignmentSubmissionResultRequest{
		AssignmentId: uint64(assignment.ID), StudentId: "student", AttemptId: 101, Score: 8.5,
	})
	if err != nil || updatedSubmission.Score == nil || *updatedSubmission.Score != 8.5 || len(publishedEvents) != 2 {
		t.Fatalf("update submission realtime event: response=%#v events=%d err=%v", updatedSubmission, len(publishedEvents), err)
	}
	updatedEvent, ok := publishedEvents[1].(classroomsSocket.GradebookUpdatedEvent)
	if !ok || updatedEvent.Payload.Status != string(model.SubmissionStatusSubmitted) || updatedEvent.Payload.Score == nil || *updatedEvent.Payload.Score != 8.5 {
		t.Fatalf("unexpected submitted gradebook event: %#v", publishedEvents[1])
	}

	identityClient, examsClient := classroomTestClients(t)
	svc := service.New(repo, &config.AppConfig{}, identityClient, examsClient)
	classGradebook, err := svc.GetClassGradebook(ctx, classroom.TeacherID, classroom.ID, dto.GradebookRangeQuery{})
	if err != nil || len(classGradebook.Students) != 1 || classGradebook.AverageScore == nil || *classGradebook.AverageScore != 8.5 {
		t.Fatalf("class gradebook: response=%#v err=%v", classGradebook, err)
	}
	progress, err := svc.GetStudentProgress(ctx, classroom.TeacherID, classroom.ID, "student")
	if err != nil || len(progress.Assignments) != 1 || progress.CompletionRate != 100 {
		t.Fatalf("student progress: response=%#v err=%v", progress, err)
	}
	analytics, err := svc.GetAssignmentAnalytics(ctx, classroom.TeacherID, classroom.ID, assignment.ID)
	if err != nil || analytics.Submitted != 1 || len(analytics.Questions) != 1 || analytics.Questions[0].WrongRate != 100 {
		t.Fatalf("assignment analytics: response=%#v err=%v", analytics, err)
	}
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
