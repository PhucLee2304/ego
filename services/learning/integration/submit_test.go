//go:build integration

package integration_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"

	"gorm.io/gorm"
)

type publishedEvent struct {
	userID    string
	eventType string
}

type attemptEvents struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (e *attemptEvents) publish(userID string, payload any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, publishedEvent{userID: userID, eventType: pushType(payload)})
}

func (e *attemptEvents) types() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	types := make([]string, len(e.events))
	for i, event := range e.events {
		types[i] = event.eventType
	}
	return types
}

func newSubmitService(db *gorm.DB, events *attemptEvents) service.Service {
	return service.New(repository.NewRepository(db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, events.publish)
}

func startTOEICTest(t *testing.T, svc service.Service, examID uint, userID string) *dto.AttemptResponse {
	t.Helper()
	attempt, httpErr := svc.CreateAttempt(context.Background(), userID, examID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	return attempt
}

func expireAttempt(t *testing.T, db *gorm.DB, attemptID uint) {
	t.Helper()
	if err := db.Model(&model.Attempt{}).Where("id = ?", attemptID).Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
}

// Nộp tay: body thay toàn bộ đáp án đã lưu, chấm điểm TOEIC, lưu history, push
// ATTEMPT_SUBMITTED; nộp lại trả nguyên history.
func TestSubmitAttemptGradesAndStoresHistory(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	events := &attemptEvents{}
	svc := newSubmitService(db, events)
	attempt := startTOEICTest(t, svc, exam.ID, "student-1")

	group := attempt.Sections[0].Groups[0]
	saved := group.Questions[1].Options[0].ID
	if _, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, group.Questions[1].ID, &saved); err != nil {
		t.Fatal(err)
	}
	correct := group.Questions[0].Options[0].ID
	submitted, httpErr := svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{
		{QuestionID: group.Questions[0].ID, SelectedOptionID: &correct},
		{QuestionID: group.Questions[1].ID},
	}})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	// Câu đã lưu qua WS nhưng body gửi null bị bỏ: chỉ còn 1 câu đúng ở listening.
	if submitted.Status != "SUBMITTED" || *submitted.CorrectAnswers != 1 || *submitted.Score != 10 || submitted.AnsweredCount != 1 {
		t.Fatalf("submitted = %#v", submitted)
	}
	answered := submitted.Sections[0].Groups[0].Questions[0]
	if answered.CorrectOptionID == nil || answered.IsCorrect == nil || !*answered.IsCorrect {
		t.Fatalf("history must reveal results: %#v", answered)
	}
	if got := events.types(); len(got) != 1 || got[0] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("events = %v", got)
	}

	again, httpErr := svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{})
	if httpErr != nil || again.ID != attempt.ID || *again.Score != *submitted.Score {
		t.Fatalf("submit again = %#v err=%v", again, httpErr)
	}
	history, httpErr := svc.GetAttemptHistory(ctx, "student-1", attempt.ID)
	if httpErr != nil || history.Status != "SUBMITTED" {
		t.Fatalf("history = %#v err=%v", history, httpErr)
	}
	if _, httpErr := svc.CancelAttempt(ctx, "student-1", attempt.ID); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("cancelling a submitted attempt must be 409, got %v", httpErr)
	}
}

func TestSubmitAttemptRejections(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newSubmitService(db, &attemptEvents{})
	attempt := startTOEICTest(t, svc, exam.ID, "student-1")
	question := attempt.Sections[1].Questions[0]

	otherOption := question.Options[0].ID + 1000
	if _, httpErr := svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{{QuestionID: question.ID, SelectedOptionID: &otherOption}}}); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("foreign option must be 400, got %v", httpErr)
	}
	if _, httpErr := svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{{QuestionID: question.ID + 1000}}}); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("foreign question must be 400, got %v", httpErr)
	}

	expireAttempt(t, db, attempt.ID)
	if _, httpErr := svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{}); httpErr == nil || httpErr.Message != service.ErrAttemptExpired.Message {
		t.Fatalf("manual submit after expiry must be rejected, got %v", httpErr)
	}

	cancelled := startTOEICTest(t, svc, exam.ID, "student-2")
	if _, httpErr := svc.CancelAttempt(ctx, "student-2", cancelled.ID); httpErr != nil {
		t.Fatal(httpErr)
	}
	if _, httpErr := svc.SubmitAttempt(ctx, "student-2", cancelled.ID, dto.SubmitAttemptRequest{}); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("submitting a cancelled attempt must be 409, got %v", httpErr)
	}
}

// Worker auto-submit: chấm đáp án đã lưu của attempt hết hạn, push
// ATTEMPT_SUBMITTING rồi ATTEMPT_SUBMITTED; attempt chưa hết hạn không bị đụng.
func TestAutoSubmitExpiredAttempts(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	events := &attemptEvents{}
	svc := newSubmitService(db, events)
	expired := startTOEICTest(t, svc, exam.ID, "student-1")
	running := startTOEICTest(t, svc, exam.ID, "student-2")

	reading := expired.Sections[1].Questions[0]
	if _, err := svc.UpdateAttemptAnswer(ctx, "student-1", expired.ID, reading.ID, &reading.Options[0].ID); err != nil {
		t.Fatal(err)
	}
	expireAttempt(t, db, expired.ID)

	submitted, err := svc.AutoSubmitExpiredAttempts(ctx, 100)
	if err != nil || submitted != 1 {
		t.Fatalf("submitted = %d err=%v", submitted, err)
	}
	var after model.Attempt
	db.First(&after, expired.ID)
	if after.Status != model.AttemptStatusSubmitted || *after.CorrectAnswers != 1 || *after.Score != 10 {
		t.Fatalf("expired attempt = %#v", after)
	}
	var stillRunning model.Attempt
	db.First(&stillRunning, running.ID)
	if stillRunning.Status != model.AttemptStatusInProgress {
		t.Fatalf("running attempt must stay ACTIVE, got %s", stillRunning.Status)
	}
	if got := events.types(); len(got) != 2 || got[0] != "ATTEMPT_SUBMITTING" || got[1] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("events = %v", got)
	}
	if submitted, err := svc.AutoSubmitExpiredAttempts(ctx, 100); err != nil || submitted != 0 {
		t.Fatalf("second scan = %d err=%v", submitted, err)
	}
}

// Lazy auto-submit: lấy câu hỏi của attempt hết hạn thì nộp luôn; tạo attempt
// mới khi attempt cũ hết hạn thì nộp cái cũ rồi tạo được cái mới.
func TestLazyAutoSubmit(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newSubmitService(db, &attemptEvents{})

	first := startTOEICTest(t, svc, exam.ID, "student-1")
	expireAttempt(t, db, first.ID)
	if _, httpErr := svc.GetAttemptQuestions(ctx, "student-1", first.ID); httpErr == nil || httpErr.Message != service.ErrAttemptExpired.Message {
		t.Fatalf("questions of expired attempt must be rejected as expired, got %v", httpErr)
	}
	if history, httpErr := svc.GetAttemptHistory(ctx, "student-1", first.ID); httpErr != nil || history.Status != "SUBMITTED" {
		t.Fatalf("expired attempt must be submitted, history=%#v err=%v", history, httpErr)
	}

	second := startTOEICTest(t, svc, exam.ID, "student-1")
	expireAttempt(t, db, second.ID)
	third := startTOEICTest(t, svc, exam.ID, "student-1")
	var previous model.Attempt
	db.First(&previous, second.ID)
	if previous.Status != model.AttemptStatusSubmitted || third.Status != "ACTIVE" {
		t.Fatalf("previous = %s, new = %s", previous.Status, third.Status)
	}
}
