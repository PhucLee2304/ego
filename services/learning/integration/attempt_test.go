//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"

	"gorm.io/gorm"
)

func toeicQuestion(part model.PartCode, order int) model.Question {
	text := "option"
	return model.Question{
		Content: "question", Part: &part, Order: order, Explanation: "secret explanation",
		Options: []model.Option{
			{Key: model.OptionA, Content: &text, IsCorrect: true, Order: 1},
			{Key: model.OptionB, Content: &text, Order: 2},
		},
	}
}

// seedTOEIC tạo đề TOEIC public: LISTENING có 1 nhóm Part 3 (2 câu) + 1 câu Part 2,
// READING có 1 câu Part 5.
func seedTOEIC(t *testing.T, db *gorm.DB) *model.Exam {
	t.Helper()
	exam := &model.Exam{
		Title: "TOEIC 1", Type: model.ExamTypeTOEIC, IsPublic: true,
		Sections: []model.Section{
			{Code: model.SectionCodeListening, Title: "Listening", Order: 1, Questions: []model.Question{toeicQuestion(model.Part2, 1)}},
			{Code: model.SectionCodeReading, Title: "Reading", Order: 2, Questions: []model.Question{toeicQuestion(model.Part5, 1)}},
		},
	}
	if err := db.Create(exam).Error; err != nil {
		t.Fatal(err)
	}
	listeningID := exam.Sections[0].ID
	group := &model.Group{Title: "Part 3", Transcript: "secret transcript", Order: 1, SectionID: listeningID}
	if err := db.Create(group).Error; err != nil {
		t.Fatal(err)
	}
	grouped := []model.Question{toeicQuestion(model.Part3, 2), toeicQuestion(model.Part3, 3)}
	for i := range grouped {
		grouped[i].SectionID, grouped[i].GroupID = listeningID, &group.ID
	}
	if err := db.Create(&grouped).Error; err != nil {
		t.Fatal(err)
	}
	return exam
}

func newAttemptService(db *gorm.DB) service.Service {
	return service.New(repository.NewRepository(db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, nil, nil)
}

// Luồng attempt standalone: TEST cả đề, chặn attempt thứ hai, ẩn lời giải khi
// đang làm, hủy bài thành history CANCELLED.
func TestStandaloneAttemptLifecycle(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newAttemptService(db)

	attempt, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if attempt.Status != "ACTIVE" || attempt.TotalQuestions != 4 || attempt.Duration == nil || *attempt.Duration != 120 || attempt.ExpiresAt == nil {
		t.Fatalf("attempt = %#v", attempt)
	}
	if wantExpiry := attempt.StartedAt.Add(120 * time.Minute); !attempt.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expiresAt = %s, want %s", attempt.ExpiresAt, wantExpiry)
	}

	if _, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone}); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("second active attempt must be 409, got %v", httpErr)
	}

	questions, httpErr := svc.GetAttemptQuestions(ctx, "student-1", attempt.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if len(questions.Sections) != 2 || questions.Sections[0].Code != "LISTENING" {
		t.Fatalf("sections = %#v", questions.Sections)
	}
	group := questions.Sections[0].Groups[0]
	if group.Transcript != "" || group.Questions[0].Explanation != "" || group.Questions[0].CorrectOptionID != nil {
		t.Fatalf("solutions must be hidden while ACTIVE: %#v", group)
	}
	if _, httpErr := svc.GetAttemptQuestions(ctx, "student-2", attempt.ID); httpErr == nil || httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("other user must get 404, got %v", httpErr)
	}

	query := dto.GetAttemptsQuery{Status: model.AttemptStatusInProgress}
	query.Normalize()
	active, _, httpErr := svc.GetAttempts(ctx, "student-1", query)
	if httpErr != nil || len(active) != 1 || active[0].ID != attempt.ID || active[0].Exam == nil {
		t.Fatalf("active attempts = %#v err=%v", active, httpErr)
	}

	cancelled, httpErr := svc.CancelAttempt(ctx, "student-1", attempt.ID)
	if httpErr != nil || cancelled.Status != "CANCELLED" || cancelled.Score != nil {
		t.Fatalf("cancel = %#v err=%v", cancelled, httpErr)
	}
	// Hủy lại trả nguyên history đã lưu.
	again, httpErr := svc.CancelAttempt(ctx, "student-1", attempt.ID)
	if httpErr != nil || again.ID != attempt.ID || again.Status != "CANCELLED" {
		t.Fatalf("cancel again = %#v err=%v", again, httpErr)
	}
	history, httpErr := svc.GetAttemptHistory(ctx, "student-1", attempt.ID)
	if httpErr != nil || history.Status != "CANCELLED" || len(history.Sections) != 2 {
		t.Fatalf("history = %#v err=%v", history, httpErr)
	}
	if _, httpErr := svc.GetAttemptQuestions(ctx, "student-1", attempt.ID); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("questions of cancelled attempt must be 400, got %v", httpErr)
	}
}

// PRACTICE TOEIC chọn part: chỉ đếm và trả câu của part đã chọn.
func TestPracticeAttemptFiltersParts(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newAttemptService(db)
	duration := model.AttemptDuration(30)

	attempt, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{
		Mode: model.AttemptModePractice, ContextType: model.AttemptContextTypeStandalone,
		Parts: []model.PartCode{model.Part3, model.Part5}, Duration: &duration,
	})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if attempt.TotalQuestions != 3 || len(attempt.Parts) != 2 || *attempt.Duration != 30 {
		t.Fatalf("attempt = %#v", attempt)
	}
	if len(attempt.Sections) != 2 || len(attempt.Sections[0].Questions) != 0 || len(attempt.Sections[0].Groups[0].Questions) != 2 {
		t.Fatalf("sections must only contain Part 3 and Part 5: %#v", attempt.Sections)
	}

	if _, httpErr := svc.CreateAttempt(ctx, "student-2", exam.ID, dto.CreateExamAttemptRequest{
		Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone, Parts: []model.PartCode{model.Part1},
	}); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("TEST with parts must be 400, got %v", httpErr)
	}
}
