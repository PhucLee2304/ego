package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
)

// fakeCorrectionRepository giữ một đề private 1 câu (option 1 đúng) và một bài đã
// nộp chọn option 2; UpdateQuestionScoring sửa trực tiếp đề dùng chung.
type fakeCorrectionRepository struct {
	repository.Repository
	exam          *model.Exam
	attempts      []*model.Attempt
	scoring       []uint
	replaced      map[uint][]model.AttemptAnswer
	updated       []*model.Attempt
	notifications []*model.NotificationOutbox
	histories     []*model.History
	audits        []*model.ExamAudit
}

func newFakeCorrectionRepository() *fakeCorrectionRepository {
	first, second := "a", "b"
	question := model.Question{Content: "Q", Explanation: "why", Options: []model.Option{
		{Key: model.OptionA, Content: &first, IsCorrect: true, Order: 1},
		{Key: model.OptionB, Content: &second, Order: 2},
	}}
	question.ID, question.Options[0].ID, question.Options[1].ID = 11, 101, 102
	exam := &model.Exam{Title: "Quiz", Type: model.ExamTypeAssignment, Sections: []model.Section{{Questions: []model.Question{question}}}}
	exam.ID = 7

	score, correct, selected, wasCorrect := 0.0, 0, uint(102), false
	attempt := &model.Attempt{
		Status: model.AttemptStatusSubmitted, UserID: "student-1", TotalQuestions: 1, Score: &score, CorrectAnswers: &correct,
		Exam:    *exam,
		Answers: []model.AttemptAnswer{{QuestionID: 11, SelectedOptionID: &selected, IsCorrect: &wasCorrect}},
	}
	attempt.ID = 3
	// Bài nộp dùng chung slice câu hỏi với đề để thấy đáp án mới sau UpdateQuestionScoring.
	attempt.Exam.Sections = exam.Sections
	return &fakeCorrectionRepository{exam: exam, attempts: []*model.Attempt{attempt}, replaced: map[uint][]model.AttemptAnswer{}}
}

func (f *fakeCorrectionRepository) Transaction(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

func (f *fakeCorrectionRepository) GetPrivateAssignmentExam(context.Context, uint) (*model.Exam, error) {
	return f.exam, nil
}

func (f *fakeCorrectionRepository) UpdateQuestionScoring(_ context.Context, questionID uint, excluded bool, correctOptionID uint) error {
	f.scoring = append(f.scoring, correctOptionID)
	question := &f.exam.Sections[0].Questions[0]
	question.ExcludedFromScore = excluded
	for i := range question.Options {
		question.Options[i].IsCorrect = question.Options[i].ID == correctOptionID
	}
	return nil
}

func (f *fakeCorrectionRepository) GetSubmittedAttemptsByExamID(context.Context, uint) ([]*model.Attempt, error) {
	return f.attempts, nil
}

func (f *fakeCorrectionRepository) ReplaceAttemptAnswers(_ context.Context, attemptID uint, answers []model.AttemptAnswer) error {
	f.replaced[attemptID] = answers
	return nil
}

func (f *fakeCorrectionRepository) UpdateAttempt(_ context.Context, attempt *model.Attempt) error {
	f.updated = append(f.updated, attempt)
	return nil
}

func (f *fakeCorrectionRepository) CreateNotificationOutbox(_ context.Context, event *model.NotificationOutbox) error {
	f.notifications = append(f.notifications, event)
	return nil
}

func (f *fakeCorrectionRepository) UpsertHistory(_ context.Context, history *model.History) error {
	f.histories = append(f.histories, history)
	return nil
}

func (f *fakeCorrectionRepository) CreateExamAudit(_ context.Context, audit *model.ExamAudit) error {
	f.audits = append(f.audits, audit)
	return nil
}

func correctionRequest(correctIndex int) dto.UpdatePrivateExamRequest {
	explanation := "why"
	return dto.UpdatePrivateExamRequest{
		Title: "Quiz", CorrectionOnly: true, ActorID: "teacher-1", Reason: " wrong key ",
		Questions: []dto.UpdatePrivateExamQuestionRequest{{
			ID: 11, Content: "Q", Explanation: &explanation, Options: []string{"a", "b"}, OptionIDs: []uint{101, 102}, CorrectOptionIndex: correctIndex,
		}},
	}
}

func TestCorrectPrivateExamRegradesSubmittedAttempts(t *testing.T) {
	repo := newFakeCorrectionRepository()
	if _, err := (&service{repo: repo}).updatePrivateExam(context.Background(), 7, correctionRequest(1)); err != nil {
		t.Fatal(err)
	}

	if len(repo.scoring) != 1 || repo.scoring[0] != 102 {
		t.Fatalf("scoring = %v", repo.scoring)
	}
	answers := repo.replaced[3]
	if len(answers) != 1 || !*answers[0].IsCorrect {
		t.Fatalf("answers must be regraded as correct: %#v", answers)
	}
	attempt := repo.updated[0]
	if *attempt.CorrectAnswers != 1 || *attempt.Score != 10 {
		t.Fatalf("attempt = correct %d score %v", *attempt.CorrectAnswers, *attempt.Score)
	}
	if len(repo.notifications) != 1 || repo.notifications[0].Source != model.NotificationSourceExam ||
		!strings.HasPrefix(repo.notifications[0].EventKey, "exam:7:regrade:attempt:3:") || repo.notifications[0].Recipients[0] != "student-1" {
		t.Fatalf("notifications = %#v", repo.notifications)
	}
	if len(repo.histories) != 1 || repo.histories[0].AttemptID != 3 {
		t.Fatalf("histories = %#v", repo.histories)
	}

	if len(repo.audits) != 1 {
		t.Fatalf("audits = %d", len(repo.audits))
	}
	audit := repo.audits[0]
	var changes []model.ExamAuditScoreChange
	if err := json.Unmarshal(audit.ScoreChanges, &changes); err != nil {
		t.Fatal(err)
	}
	if audit.ActorID != "teacher-1" || audit.Reason != "wrong key" || audit.RegradedAt == nil ||
		len(changes) != 1 || *changes[0].BeforeScore != 0 || *changes[0].AfterScore != 10 || changes[0].StudentID != "student-1" {
		t.Fatalf("audit = %#v changes = %#v", audit, changes)
	}
}

func TestCorrectPrivateExamRejectsStructuralChanges(t *testing.T) {
	missingReason := correctionRequest(1)
	missingReason.Reason = " "
	renamed := correctionRequest(1)
	renamed.Title = "Renamed"
	editedContent := correctionRequest(1)
	editedContent.Questions[0].Content = "Q2"
	editedOption := correctionRequest(1)
	editedOption.Questions[0].Options = []string{"a", "c"}
	badCorrect := correctionRequest(5)

	tests := []struct {
		name    string
		req     dto.UpdatePrivateExamRequest
		message string
	}{
		{"missing reason", missingReason, "[ERROR] Correction actor and reason are required"},
		{"title changed", renamed, "[ERROR] An exam with attempts only allows correcting the answer or changing score exclusion"},
		{"question content changed", editedContent, "[ERROR] Question structure cannot be changed after an attempt exists"},
		{"option changed", editedOption, "[ERROR] Question options cannot be changed after an attempt exists"},
		{"correct option out of range", badCorrect, "[ERROR] Correct option is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeCorrectionRepository()
			_, err := (&service{repo: repo}).updatePrivateExam(context.Background(), 7, tt.req)
			var httpErr *httpx.HTTPError
			if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest || httpErr.Message != tt.message {
				t.Fatalf("err = %v, want 400 %q", err, tt.message)
			}
			if len(repo.audits) != 0 || len(repo.updated) != 0 {
				t.Fatal("rejected correction must not regrade or audit")
			}
		})
	}
}
