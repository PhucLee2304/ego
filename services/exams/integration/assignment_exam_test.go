//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/exams/database"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/model"
	"ego/services/exams/internal/repository"
	"ego/services/exams/internal/service"
	"google.golang.org/grpc"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fakeNotificationClient struct{ calls int }

func (f *fakeNotificationClient) CreateNotifications(_ context.Context, req *notificationsClient.CreateNotificationsRequest, _ ...grpc.CallOption) (*notificationsClient.CreateNotificationsResponse, error) {
	f.calls++
	return &notificationsClient.CreateNotificationsResponse{CreatedCount: uint32(len(req.Recipients))}, nil
}

func TestPrivateAssignmentExamConstraints(t *testing.T) {
	option := func(key model.OptionKey, content string, correct bool) model.Option {
		return model.Option{Key: key, Content: &content, IsCorrect: correct}
	}
	db, err := gorm.Open(postgres.Open(os.Getenv("EXAMS_TEST_DSN")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}

	exam := &model.Exam{Title: "Integration assignment", Type: model.ExamTypeAssignment, IsPublic: false,
		Sections: []model.Section{{Code: model.SectionCodeFull, Title: "Questions", Order: 1,
			Questions: []model.Question{{Content: "2 + 2 = ?", Order: 1, Options: []model.Option{
				option(model.OptionA, "1", false), option(model.OptionB, "2", false),
				option(model.OptionC, "3", false), option(model.OptionD, "4", true),
			}}},
		}},
	}
	if err := db.Create(exam).Error; err != nil {
		t.Fatalf("create private exam tree: %v", err)
	}
	if exam.ID == 0 || exam.IsPublic {
		t.Fatalf("unexpected private exam: %#v", exam)
	}
	repo := repository.NewRepository(db)
	if err := repo.CreateNotificationOutbox(context.Background(), &model.NotificationOutbox{EventKey: "attempt:test", Recipients: datatypes.JSONSlice[string]{"student"}, Title: "Graded", Body: "Body", Data: datatypes.JSONMap{"id": "1", "type": "ATTEMPT"}, NextAttemptAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	notificationClient := &fakeNotificationClient{}
	notificationSvc := service.New(repo, nil)
	notificationSvc.SetNotificationClient(notificationClient)
	if processed, err := notificationSvc.ProcessNotificationOutbox(context.Background(), 10); err != nil || processed != 1 || notificationClient.calls != 1 {
		t.Fatalf("process notification outbox: processed=%d calls=%d err=%v", processed, notificationClient.calls, err)
	}
	if processed, err := notificationSvc.ProcessNotificationOutbox(context.Background(), 10); err != nil || processed != 0 || notificationClient.calls != 1 {
		t.Fatalf("processed event was retried: processed=%d calls=%d err=%v", processed, notificationClient.calls, err)
	}
	if _, err := repo.GetExamByID(context.Background(), exam.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("private exam must not be returned by public lookup, got %v", err)
	}

	question := exam.Sections[0].Questions[0]
	if err := db.Model(&model.Option{}).Where("question_id = ? AND key = ?", question.ID, "A").Update("is_correct", true).Error; err == nil {
		t.Fatal("expected the database to reject two correct options")
	}

	assignmentID := uint(72)
	now := time.Now()
	attempt := &model.Attempt{
		Mode: model.AttemptModeAssignment, Status: model.AttemptStatusInProgress,
		StartedAt: now, TotalQuestions: 1, ExamID: exam.ID, UserID: "student",
		ContextType: model.AttemptContextTypeClassroomAssignment, ContextID: &assignmentID,
	}
	if err := db.Create(attempt).Error; err != nil {
		t.Fatal(err)
	}
	dueAt := now.Add(15 * time.Minute)
	updated, err := repo.UpdateActiveAssignmentAttemptSchedule(context.Background(), assignmentID, 30, dueAt)
	if err != nil || updated != 1 {
		t.Fatalf("update active attempt schedule: rows=%d err=%v", updated, err)
	}
	var scheduled model.Attempt
	if err := db.First(&scheduled, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	expiresAtMismatch := scheduled.ExpiresAt == nil || scheduled.ExpiresAt.Sub(dueAt).Abs() >= time.Millisecond
	if scheduled.Duration == nil || *scheduled.Duration != 30 || expiresAtMismatch {
		t.Fatalf("attempt schedule was not synchronized: %#v", scheduled)
	}
	correctOptionID := question.Options[3].ID
	if err := db.Create(&model.AttemptAnswer{AttemptID: attempt.ID, QuestionID: question.ID, SelectedOptionID: &correctOptionID}).Error; err != nil {
		t.Fatal(err)
	}

	svc := service.New(repo, nil)
	var attemptEvents []string
	svc.SetAttemptEventPublisher(func(userID string, payload any) {
		if userID != "student" {
			t.Fatalf("event sent to unexpected user %q", userID)
		}
		message, ok := payload.(map[string]any)
		if !ok {
			t.Fatalf("unexpected event payload: %#v", payload)
		}
		attemptEvents = append(attemptEvents, message["type"].(string))
	})
	count, err := svc.SubmitAssignmentAttempts(context.Background(), assignmentID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one submitted attempt, got %d", count)
	}
	if len(attemptEvents) != 2 || attemptEvents[0] != "ATTEMPT_SUBMITTING" || attemptEvents[1] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("unexpected attempt event sequence: %#v", attemptEvents)
	}
	var saved model.Attempt
	if err := db.Preload("History").First(&saved, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.AttemptStatusSubmitted || saved.Score == nil || *saved.Score != 10 || saved.History == nil {
		t.Fatalf("attempt was not fully submitted: %#v", saved)
	}
	emptyExplanation := ""
	optionIDs := make([]uint, len(question.Options))
	for i := range question.Options {
		optionIDs[i] = question.Options[i].ID
	}
	updatedExam, err := svc.UpdatePrivateExam(context.Background(), exam.ID, dto.UpdatePrivateExamRequest{
		Title: exam.Title, Description: exam.Description, CorrectionOnly: true, ActorID: "teacher-1", Reason: "Correct an invalid answer key",
		Questions: []dto.UpdatePrivateExamQuestionRequest{{
			ID: question.ID, Content: question.Content, Explanation: &emptyExplanation,
			Options: []string{"1", "2", "3", "4"}, OptionIDs: optionIDs, CorrectOptionIndex: 0,
		}},
	})
	if err != nil {
		t.Fatalf("correct private exam after submission: %v", err)
	}
	if updatedExam.Sections[0].Questions[0].Options[0].IsCorrect != true {
		t.Fatalf("private exam answer was not corrected: %#v", updatedExam)
	}
	var audit model.ExamAudit
	if err := db.Where("exam_id = ?", exam.ID).Order("id DESC").First(&audit).Error; err != nil {
		t.Fatalf("correction audit was not stored: %v", err)
	}
	if audit.ActorID != "teacher-1" || audit.Reason != "Correct an invalid answer key" || len(audit.Before) == 0 || len(audit.After) == 0 || audit.RegradedAt == nil {
		t.Fatalf("correction audit is incomplete: %#v", audit)
	}
	var scoreChanges []model.ExamAuditScoreChange
	if err := json.Unmarshal(audit.ScoreChanges, &scoreChanges); err != nil {
		t.Fatalf("decode audit score changes: %v", err)
	}
	if len(scoreChanges) != 1 || scoreChanges[0].AttemptID != attempt.ID || scoreChanges[0].BeforeScore == nil || *scoreChanges[0].BeforeScore != 10 || scoreChanges[0].AfterScore == nil || *scoreChanges[0].AfterScore != 0 {
		t.Fatalf("unexpected audit score changes: %#v", scoreChanges)
	}
	questionAnalytics, err := svc.GetAssignmentQuestionAnalytics(context.Background(), assignmentID, exam.ID)
	if err != nil || len(questionAnalytics) != 1 || questionAnalytics[0].AnsweredCount != 1 || questionAnalytics[0].WrongCount != 1 {
		t.Fatalf("assignment question analytics: rows=%#v err=%v", questionAnalytics, err)
	}
	var preservedHistory model.History
	if err := db.First(&preservedHistory, saved.History.ID).Error; err != nil {
		t.Fatalf("submitted history must survive exam replacement: %v", err)
	}
	regraded, err := dto.UnmarshalAttemptHistory(preservedHistory.Data)
	if err != nil || regraded.Score == nil || *regraded.Score != 0 {
		t.Fatalf("history was not regraded: response=%#v err=%v", regraded, err)
	}
	if _, err := svc.UpdatePrivateExam(context.Background(), exam.ID, dto.UpdatePrivateExamRequest{
		Title: exam.Title, Description: exam.Description, CorrectionOnly: true,
		Questions: []dto.UpdatePrivateExamQuestionRequest{{ID: question.ID, Content: "Changed content", Explanation: &emptyExplanation, Options: []string{"1", "2", "3", "4"}, OptionIDs: optionIDs, CorrectOptionIndex: 0}},
	}); err == nil {
		t.Fatal("expected structural correction to be rejected after submission")
	}

	secondAssignmentID := uint(73)
	secondAttempt := &model.Attempt{
		Mode: model.AttemptModeAssignment, Status: model.AttemptStatusInProgress,
		StartedAt: now, TotalQuestions: 1, ExamID: exam.ID, UserID: "student-2",
		ContextType: model.AttemptContextTypeClassroomAssignment, ContextID: &secondAssignmentID,
	}
	if err := db.Create(secondAttempt).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AttemptAnswer{AttemptID: secondAttempt.ID, QuestionID: question.ID, SelectedOptionID: &correctOptionID}).Error; err != nil {
		t.Fatal(err)
	}
	svc.SetAttemptEventPublisher(nil)
	count, err = svc.SubmitStudentAssignmentAttempts(context.Background(), "student-2", []uint{secondAssignmentID})
	if err != nil || count != 1 {
		t.Fatalf("submit removed student's active attempt: count=%d err=%v", count, err)
	}
	if err := db.First(&secondAttempt, secondAttempt.ID).Error; err != nil || secondAttempt.Status != model.AttemptStatusSubmitted {
		t.Fatalf("student attempt was not submitted: attempt=%#v err=%v", secondAttempt, err)
	}

	aggregateID := "attempt-72"
	nextAttemptAt := time.Now()
	event := &model.OutboxEvent{
		Type: model.OutboxEventTypeClassroomAssignmentAttemptSubmitted, AggregateID: &aggregateID,
		Status: model.OutboxEventStatusFailedPermanent, Payload: []byte(`{"attemptId":72}`),
		Attempts: 10, NextAttemptAt: nextAttemptAt,
	}
	if err := db.Create(event).Error; err != nil {
		t.Fatal(err)
	}
	lastError := "classrooms unavailable"
	if err := db.Model(event).Updates(map[string]any{"last_error": lastError}).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.RequeueOrCreateOutboxEvent(context.Background(), &model.OutboxEvent{
		Type: event.Type, AggregateID: &aggregateID, Payload: []byte(`{"attemptId":72,"retry":true}`),
		Status: model.OutboxEventStatusPending, NextAttemptAt: nextAttemptAt.Add(time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	var requeued model.OutboxEvent
	if err := db.First(&requeued, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requeued.Status != model.OutboxEventStatusPending || requeued.Attempts != 0 || requeued.ProcessedAt != nil || requeued.LastError == nil || *requeued.LastError != "" {
		t.Fatalf("permanent outbox event was not reset: %#v", requeued)
	}

	part := model.Part3
	groupExam := &model.Exam{
		Title: "TOEIC grouped questions", Type: model.ExamTypeTOEIC, IsPublic: true,
	}
	if err := db.Create(groupExam).Error; err != nil {
		t.Fatal(err)
	}
	section := &model.Section{Code: model.SectionCodeListening, Title: "Listening", Order: 1, ExamID: groupExam.ID}
	if err := db.Create(section).Error; err != nil {
		t.Fatal(err)
	}
	group := &model.Group{Title: "Conversation", Order: 1, SectionID: section.ID}
	if err := db.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	groupQuestion := model.Question{
		Content: "What is discussed?", Part: &part, Order: 1,
		SectionID: section.ID, GroupID: &group.ID,
	}
	if err := db.Create(&groupQuestion).Error; err != nil {
		t.Fatal(err)
	}
	groupQuestion.Options = []model.Option{
		option(model.OptionA, "A meeting", true), option(model.OptionB, "A trip", false),
		option(model.OptionC, "A meal", false), option(model.OptionD, "A report", false),
	}
	for i := range groupQuestion.Options {
		groupQuestion.Options[i].QuestionID = groupQuestion.ID
	}
	if err := db.Create(&groupQuestion.Options).Error; err != nil {
		t.Fatal(err)
	}
	groupAttempt := &model.Attempt{
		Mode: model.AttemptModePractice, Status: model.AttemptStatusInProgress,
		StartedAt: time.Now(), TotalQuestions: 1, ExamID: groupExam.ID,
		UserID: "toeic-student", ContextType: model.AttemptContextTypeStandalone,
	}
	if err := db.Create(groupAttempt).Error; err != nil {
		t.Fatal(err)
	}
	selectedOptionID := groupQuestion.Options[0].ID
	if _, err := svc.UpdateAttemptAnswer(context.Background(), groupAttempt.UserID, groupAttempt.ID, groupQuestion.ID, &selectedOptionID); err != nil {
		t.Fatalf("update answer for grouped TOEIC question: %v", err)
	}
	var answer model.AttemptAnswer
	if err := db.Where("attempt_id = ? AND question_id = ?", groupAttempt.ID, groupQuestion.ID).First(&answer).Error; err != nil {
		t.Fatalf("grouped TOEIC answer was not persisted: %v", err)
	}
	// SubmitAttempt and CreateAttempt return *httpx.HTTPError; keep it out of the
	// error-typed err so a nil pointer is not turned into a non-nil interface.
	result, httpErr := svc.SubmitAttempt(context.Background(), groupAttempt.UserID, groupAttempt.ID, dto.SubmitAttemptRequest{
		Answers: []dto.SubmitAttemptAnswer{{QuestionID: groupQuestion.ID, SelectedOptionID: &selectedOptionID}},
	})
	if httpErr != nil {
		t.Fatalf("submit grouped TOEIC attempt: %v", httpErr)
	}
	if result.TotalQuestions != 1 || result.CorrectAnswers == nil || *result.CorrectAnswers != 1 || result.Score == nil || *result.Score != 10 {
		t.Fatalf("grouped TOEIC attempt was scored incorrectly: %#v", result)
	}

	duration := model.AttemptDuration10
	createdAttempt, httpErr := svc.CreateAttempt(context.Background(), "toeic-create-student", groupExam.ID, dto.CreateExamAttemptRequest{
		Mode:        model.AttemptModePractice,
		Parts:       []model.PartCode{model.Part3},
		Duration:    &duration,
		ContextType: model.AttemptContextTypeStandalone,
	})
	if httpErr != nil {
		t.Fatalf("create grouped TOEIC attempt: %v", httpErr)
	}
	if createdAttempt.TotalQuestions != 1 {
		t.Fatalf("expected grouped question in attempt total, got %d", createdAttempt.TotalQuestions)
	}

	publicExam, err := repo.GetExamByID(context.Background(), groupExam.ID)
	if err != nil {
		t.Fatalf("get grouped public exam: %v", err)
	}
	examResponse := dto.ToGetExamResponse(publicExam)
	if examResponse.TotalQuestions != 1 || len(examResponse.Parts) != 1 || examResponse.Parts[0].Part != "PART 3" {
		t.Fatalf("grouped public exam summary is incomplete: %#v", examResponse)
	}
}
