//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
	"ego/services/learning/internal/socket"
)

// gradebookEvents ghi các GRADEBOOK_UPDATED đẩy lên hub /ws/classrooms.
type gradebookEvents struct {
	mu     sync.Mutex
	userID []string
	events []socket.GradebookUpdatedPayload
}

func (e *gradebookEvents) publish(userID string, payload any) {
	event, ok := payload.(socket.GradebookUpdatedEvent)
	if !ok {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.userID = append(e.userID, userID)
	e.events = append(e.events, event.Payload)
}

func (e *gradebookEvents) all() []socket.GradebookUpdatedPayload {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]socket.GradebookUpdatedPayload(nil), e.events...)
}

// newAssignmentAttemptFixture như newAssignmentFixture, thêm publisher hub classrooms.
func newAssignmentAttemptFixture(t *testing.T) (*assignmentFixture, *gradebookEvents) {
	t.Helper()
	f := newAssignmentFixture(t)
	gradebook := &gradebookEvents{}
	f.svc = service.New(repository.NewRepository(f.db), &config.AppConfig{}, nil, testIdentityClient{}, nil, nil, gradebook.publish, f.events.publish)
	return f, gradebook
}

func assignmentAttemptRequest(assignmentID uint) dto.CreateExamAttemptRequest {
	return dto.CreateExamAttemptRequest{Mode: model.AttemptModeAssignment, ContextType: model.AttemptContextTypeClassroomAssignment, ContextID: &assignmentID}
}

// Tạo attempt cho assignment: kiểm tra assignment, thời hạn theo assignment, và
// submission ghi cùng tx với attempt; nộp bài ghi điểm vào submission cùng tx.
func TestAssignmentAttemptLifecycle(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	created := f.create(t, f.classroom.ID)
	assignment := created.Assignments[0]

	attempt, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID))
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if attempt.Duration == nil || *attempt.Duration != 45 || attempt.ExpiresAt == nil || attempt.ExpiresAt.Sub(time.Now().Add(45*time.Minute)).Abs() > time.Minute {
		t.Fatalf("attempt must use the assignment duration: %#v", attempt)
	}
	var submission model.AssignmentSubmission
	if err := f.db.Where("assignment_id = ? AND student_id = ?", assignment.ID, "student-1").First(&submission).Error; err != nil {
		t.Fatalf("submission must be created with the attempt: %v", err)
	}
	if submission.AttemptID != attempt.ID || submission.Score != nil {
		t.Fatalf("submission = %#v", submission)
	}
	if events := gradebook.all(); len(events) != 1 || events[0].Status != string(model.SubmissionStatusInProgress) || events[0].AttemptID != attempt.ID || gradebook.userID[0] != "teacher-1" {
		t.Fatalf("gradebook events after create = %#v to %v", events, gradebook.userID)
	}

	if _, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID)); httpErr == nil || httpErr.StatusCode != http.StatusConflict {
		t.Fatalf("second attempt must conflict, got %v", httpErr)
	}

	result, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{}})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if err := f.db.Where("assignment_id = ? AND student_id = ?", assignment.ID, "student-1").First(&submission).Error; err != nil {
		t.Fatal(err)
	}
	if submission.Score == nil || result.Score == nil || *submission.Score != *result.Score {
		t.Fatalf("submission score must match attempt score: submission=%#v result=%#v", submission, result)
	}
	if events := gradebook.all(); len(events) != 2 || events[1].Status != string(model.SubmissionStatusSubmitted) || events[1].Score == nil {
		t.Fatalf("gradebook events after submit = %#v", events)
	}
}

// Lỗi kiểm tra assignment giữ đúng status + message như khi còn đi qua gRPC.
func TestAssignmentAttemptValidationErrors(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	other := f.create(t, f.other.ID).Assignments[0]

	tests := []struct {
		name         string
		userID       string
		examID       uint
		assignmentID uint
		status       int
		message      string
	}{
		{"assignment not found", "student-1", assignment.ExamID, 9999, http.StatusNotFound, "[ERROR] Assignment not found"},
		{"exam mismatch", "student-1", other.ExamID, assignment.ID, http.StatusBadRequest, "[ERROR] Assignment does not belong to exam"},
		{"student not approved", "student-2", assignment.ExamID, assignment.ID, http.StatusForbidden, "[FORBIDDEN] Student is not approved in classroom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, httpErr := f.svc.CreateAttempt(ctx, tt.userID, tt.examID, assignmentAttemptRequest(tt.assignmentID))
			if httpErr == nil || httpErr.StatusCode != tt.status || httpErr.Message != tt.message {
				t.Fatalf("got %v, want %d %q", httpErr, tt.status, tt.message)
			}
		})
	}

	f.db.Model(&model.Assignment{}).Where("id = ?", assignment.ID).Update("active", false)
	if _, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID)); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != "[ERROR] Assignment is not active" {
		t.Fatalf("inactive assignment, got %v", httpErr)
	}
	f.db.Model(&model.Assignment{}).Where("id = ?", assignment.ID).Updates(map[string]any{"active": true, "due_at": time.Now().Add(-time.Minute)})
	if _, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID)); httpErr == nil || httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != "[ERROR] Assignment is closed" {
		t.Fatalf("closed assignment, got %v", httpErr)
	}

	var attempts, submissions int64
	f.db.Model(&model.Attempt{}).Count(&attempts)
	f.db.Model(&model.AssignmentSubmission{}).Count(&submissions)
	if attempts != 0 || submissions != 0 || len(gradebook.all()) != 0 {
		t.Fatalf("rejected attempts must write nothing: attempts=%d submissions=%d events=%d", attempts, submissions, len(gradebook.all()))
	}
}

// Ghi điểm submission lỗi thì cả bài nộp rollback: attempt vẫn đang làm, không có
// history, không push ATTEMPT_SUBMITTED/GRADEBOOK_UPDATED.
func TestAssignmentSubmitRollsBackWithSubmission(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	attempt := f.startAssignmentAttempt(t, 9999, assignment.ExamID, "student-1")

	_, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{}})
	if httpErr == nil || httpErr.StatusCode != http.StatusNotFound || httpErr.Message != "[ERROR] Assignment not found" {
		t.Fatalf("submit with missing assignment, got %v", httpErr)
	}
	var saved model.Attempt
	if err := f.db.Preload("History").First(&saved, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.AttemptStatusInProgress || saved.Score != nil || saved.History != nil {
		t.Fatalf("attempt must be rolled back: %#v", saved)
	}
	var notifications int64
	f.db.Model(&model.NotificationOutbox{}).Where("event_key = ?", fmt.Sprintf("attempt:%d:submitted", attempt.ID)).Count(&notifications)
	if notifications != 0 || len(gradebook.all()) != 0 {
		t.Fatalf("rolled back submit must not notify: notifications=%d gradebook=%d", notifications, len(gradebook.all()))
	}
	for _, eventType := range f.events.types() {
		if eventType == "ATTEMPT_SUBMITTED" {
			t.Fatal("rolled back submit must not push ATTEMPT_SUBMITTED")
		}
	}
}

// Xóa học sinh khỏi lớp nộp luôn bài assignment đang làm trong cùng tx, có điểm
// submission; push gửi sau commit.
func TestRemoveMemberSubmitsActiveAssignmentAttempts(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	attempt, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID))
	if httpErr != nil {
		t.Fatal(httpErr)
	}

	if httpErr := f.svc.RemoveMember(ctx, "teacher-1", f.classroom.ID, "student-1"); httpErr != nil {
		t.Fatal(httpErr)
	}
	var saved model.Attempt
	if err := f.db.First(&saved, attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.Status != model.AttemptStatusSubmitted {
		t.Fatalf("active attempt must be submitted on removal: %#v", saved)
	}
	var submission model.AssignmentSubmission
	if err := f.db.Where("assignment_id = ? AND student_id = ?", assignment.ID, "student-1").First(&submission).Error; err != nil || submission.Score == nil {
		t.Fatalf("submission must be graded: %#v err=%v", submission, err)
	}
	var members int64
	f.db.Model(&model.Member{}).Where("classroom_id = ? AND user_id = ?", f.classroom.ID, "student-1").Count(&members)
	if members != 0 {
		t.Fatal("member must be removed")
	}
	if events := gradebook.all(); len(events) != 2 || events[1].Status != string(model.SubmissionStatusSubmitted) {
		t.Fatalf("gradebook events = %#v", events)
	}
	if types := f.events.types(); len(types) != 2 || types[0] != "ATTEMPT_SUBMITTING" || types[1] != "ATTEMPT_SUBMITTED" {
		t.Fatalf("attempt events = %#v", types)
	}
}

// Đính chính đề cập nhật điểm submission trong cùng tx và push GRADEBOOK_UPDATED.
func TestAssignmentCorrectionRegradesSubmission(t *testing.T) {
	f, gradebook := newAssignmentAttemptFixture(t)
	ctx := context.Background()
	assignment := f.create(t, f.classroom.ID).Assignments[0]
	attempt, httpErr := f.svc.CreateAttempt(ctx, "student-1", assignment.ExamID, assignmentAttemptRequest(assignment.ID))
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	editable, httpErr := f.svc.GetEditableAssignment(ctx, "teacher-1", f.classroom.ID, assignment.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	// Chọn đáp án đúng cả 2 câu (10 điểm), rồi đính chính đáp án đúng sang B (0 điểm).
	answers := make([]dto.SubmitAttemptAnswer, len(editable.Questions))
	for i, q := range editable.Questions {
		answers[i] = dto.SubmitAttemptAnswer{QuestionID: q.ID, SelectedOptionID: &q.OptionIDs[0]}
	}
	if result, httpErr := f.svc.SubmitAttempt(ctx, "student-1", attempt.ID, dto.SubmitAttemptRequest{Answers: answers}); httpErr != nil || result.Score == nil || *result.Score != 10 {
		t.Fatalf("submit: result=%#v err=%v", result, httpErr)
	}
	questions := make([]dto.UpdateAssignmentQuestionRequest, len(editable.Questions))
	for i, q := range editable.Questions {
		explanation := ""
		if q.Explanation != nil {
			explanation = *q.Explanation
		}
		questions[i] = dto.UpdateAssignmentQuestionRequest{ID: q.ID, Content: q.Content, Explanation: &explanation, Options: q.Options, OptionIDs: q.OptionIDs, CorrectOptionIndex: 1, ExcludedFromScore: q.ExcludedFromScore}
	}
	before := len(gradebook.all())
	if _, httpErr := updateContent(ctx, f.svc, "teacher-1", f.classroom.ID, assignment.ID, dto.UpdateAssignmentContentRequest{
		Title: editable.Title, Description: editable.Description, Questions: questions, Confirm: true, Reason: "Fix key",
	}); httpErr != nil {
		t.Fatal(httpErr)
	}
	var regraded model.Attempt
	f.db.First(&regraded, attempt.ID)
	var submission model.AssignmentSubmission
	f.db.Where("assignment_id = ? AND student_id = ?", assignment.ID, "student-1").First(&submission)
	if submission.Score == nil || regraded.Score == nil || *regraded.Score != 0 || *submission.Score != 0 {
		t.Fatalf("submission score must follow regrade: submission=%#v attempt=%#v", submission, regraded)
	}
	if events := gradebook.all(); len(events) != before+1 {
		t.Fatalf("correction must push one GRADEBOOK_UPDATED, got %#v", events[before:])
	}
}
