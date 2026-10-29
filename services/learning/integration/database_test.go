//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"

	"google.golang.org/grpc"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Port từ exams TestPrivateAssignmentExamConstraints và classrooms
// TestAssignmentScheduleAndExamUniqueness: phần ràng buộc DB, outbox thông báo và
// schedule. Phần submission/gradebook/analytics đã ở assignment_attempt_test.go,
// gradebook_test.go; phần OutboxEvent bị bỏ (mục 2.5).

type fakeNotificationClient struct{ calls int }

func (f *fakeNotificationClient) CreateNotifications(_ context.Context, req *notificationsClient.CreateNotificationsRequest, _ ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error) {
	f.calls++
	return &notificationsClient.CreateNotificationsResponse{CreatedCount: uint32(len(req.Recipients))}, nil
}

// Trigger chặn đề assignment có hai đáp án đúng; index unique chặn gán một đề hai
// lần cho cùng lớp.
func TestDatabaseConstraints(t *testing.T) {
	db := learningTestDB(t)
	content := func(value string) *string { return &value }
	exam := &model.Exam{Title: "Integration assignment", Type: model.ExamTypeAssignment,
		Sections: []model.Section{{Code: model.SectionCodeFull, Title: "Questions", Order: 1,
			Questions: []model.Question{{Content: "2 + 2 = ?", Order: 1, Options: []model.Option{
				{Key: model.OptionA, Content: content("1")}, {Key: model.OptionB, Content: content("2")},
				{Key: model.OptionC, Content: content("3")}, {Key: model.OptionD, Content: content("4"), IsCorrect: true},
			}}},
		}},
	}
	if err := db.Create(exam).Error; err != nil {
		t.Fatalf("create private exam tree: %v", err)
	}
	question := exam.Sections[0].Questions[0]
	if err := db.Model(&model.Option{}).Where("question_id = ? AND key = ?", question.ID, "A").Update("is_correct", true).Error; err == nil {
		t.Fatal("expected the database to reject two correct options")
	}

	classroom := &model.Classroom{Code: "TEST0001", Name: "Test class", TeacherID: "teacher", Active: true}
	if err := db.Create(classroom).Error; err != nil {
		t.Fatal(err)
	}
	opensAt := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	assignment := &model.Assignment{ClassroomID: classroom.ID, ExamID: exam.ID, Title: "Assignment", Duration: 45, OpensAt: opensAt, DueAt: opensAt.Add(2 * time.Hour), Active: true}
	if err := db.Create(assignment).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := *assignment
	duplicate.Model = gorm.Model{}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("expected duplicate classroom/exam assignment to be rejected")
	}
}

// Một dòng outbox chỉ được gửi sang notifications một lần.
func TestNotificationOutboxIsProcessedOnce(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	repo := repository.NewRepository(db)
	if err := repo.CreateNotificationOutbox(ctx, &model.NotificationOutbox{EventKey: "attempt:test", Source: model.NotificationSourceAttempt, Recipients: datatypes.JSONSlice[string]{"student"}, Title: "Graded", Body: "Body", Data: datatypes.JSONMap{"id": "1", "type": "ATTEMPT"}, NextAttemptAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	client := &fakeNotificationClient{}
	svc := service.New(repo, &config.AppConfig{}, client, testIdentityClient{}, nil, nil, nil, nil)
	if processed, err := svc.ProcessNotificationOutbox(ctx, 10); err != nil || processed != 1 || client.calls != 1 {
		t.Fatalf("process notification outbox: processed=%d calls=%d err=%v", processed, client.calls, err)
	}
	if processed, err := svc.ProcessNotificationOutbox(ctx, 10); err != nil || processed != 0 || client.calls != 1 {
		t.Fatalf("processed event was retried: processed=%d calls=%d err=%v", processed, client.calls, err)
	}
}

// Calendar gộp schedule đã lưu với hạn nộp assignment; sửa/xóa schedule.
func TestScheduleCrudAndCalendar(t *testing.T) {
	f := newAssignmentFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	dueAt := f.opensAt.Add(48 * time.Hour)
	startsAt := f.opensAt.Add(2 * time.Hour)
	endsAt := startsAt.Add(time.Hour)

	created, httpErr := f.svc.CreateSchedule(ctx, "teacher-1", f.classroom.ID, dto.CreateScheduleRequest{Title: "Live class", StartsAt: startsAt, EndsAt: endsAt})
	if httpErr != nil || created.ID == 0 {
		t.Fatalf("create schedule: response=%#v err=%v", created, httpErr)
	}
	calendar, httpErr := f.svc.GetCalendar(ctx, "teacher-1", f.classroom.ID, dto.CalendarQuery{From: f.opensAt, To: dueAt.Add(time.Hour)})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if len(calendar) != 2 || calendar[0].CalendarType != dto.CalendarTypeClassSchedule || calendar[1].CalendarType != dto.CalendarTypeAssignmentDeadline || calendar[1].RelatedAssignmentID == nil || *calendar[1].RelatedAssignmentID != assignment.ID {
		t.Fatalf("expected stored schedule and dynamic assignment deadline, got %#v", calendar)
	}
	updated, httpErr := f.svc.UpdateSchedule(ctx, "teacher-1", created.ID, dto.UpdateScheduleRequest{Title: "Consultation", StartsAt: startsAt, EndsAt: endsAt})
	if httpErr != nil || updated.Title != "Consultation" || updated.CalendarType != dto.CalendarTypeClassSchedule {
		t.Fatalf("update schedule: response=%#v err=%v", updated, httpErr)
	}
	if httpErr := f.svc.DeleteSchedule(ctx, "teacher-1", created.ID); httpErr != nil {
		t.Fatal(httpErr)
	}
	if _, err := repository.NewRepository(f.db).GetScheduleByID(ctx, created.ID); err == nil {
		t.Fatal("expected deleted schedule to be hidden by GORM soft delete")
	}
}
