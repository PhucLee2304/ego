package service

import (
	"context"
	"errors"
	"testing"
	"time"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

type fakeAnswerRepository struct {
	repository.Repository
	attempt   *model.Attempt
	getErr    error
	upsertErr error
	upserted  []*model.AttemptAnswer
}

func (f *fakeAnswerRepository) Transaction(_ context.Context, fn func(repository.Repository) error) error {
	return fn(f)
}

func (f *fakeAnswerRepository) GetAttemptForAnswerUpdate(context.Context, uint, string) (*model.Attempt, error) {
	return f.attempt, f.getErr
}

func (f *fakeAnswerRepository) UpsertAttemptAnswer(_ context.Context, answer *model.AttemptAnswer) error {
	f.upserted = append(f.upserted, answer)
	return f.upsertErr
}

func answerTestAttempt() *model.Attempt {
	question := model.Question{Options: []model.Option{{IsCorrect: true}}}
	question.ID, question.Options[0].ID = 11, 101
	attempt := &model.Attempt{Status: model.AttemptStatusInProgress, Exam: model.Exam{Sections: []model.Section{{Questions: []model.Question{question}}}}}
	attempt.ID = 1
	return attempt
}

// Hợp đồng WS: mỗi trường hợp từ chối trả đúng một mã lỗi ATTEMPT_ANSWER_UPDATED.
func TestUpdateAttemptAnswerSocketErrorCodes(t *testing.T) {
	expired := answerTestAttempt()
	past := time.Now().Add(-time.Minute)
	expired.ExpiresAt = &past
	cancelled := answerTestAttempt()
	cancelled.Status = model.AttemptStatusCancelled

	tests := []struct {
		name       string
		repo       *fakeAnswerRepository
		questionID uint
		optionID   uint
		code       platformsocket.ErrorCode
		message    string
	}{
		{"attempt not found", &fakeAnswerRepository{getErr: gorm.ErrRecordNotFound}, 11, 101, socket.ErrorCodeAttemptNotFound, "[SOCKET] Attempt not found"},
		{"attempt not active", &fakeAnswerRepository{attempt: cancelled}, 11, 101, socket.ErrorCodeAttemptNotActive, ErrAttemptNotActive.Message},
		{"attempt expired", &fakeAnswerRepository{attempt: expired}, 11, 101, socket.ErrorCodeAttemptExpired, ErrAttemptExpired.Message},
		{"question not in attempt", &fakeAnswerRepository{attempt: answerTestAttempt()}, 12, 101, socket.ErrorCodeQuestionNotInAttempt, ErrQuestionNotBelongToAttempt.Message},
		{"option not in question", &fakeAnswerRepository{attempt: answerTestAttempt()}, 11, 102, socket.ErrorCodeOptionNotInQuestion, ErrOptionNotBelongToQuestion.Message},
		{"storage failure", &fakeAnswerRepository{attempt: answerTestAttempt(), upsertErr: errors.New("db down")}, 11, 101, socket.ErrorCodeAttemptAnswerRejected, "db down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			optionID := tt.optionID
			_, err := (&service{repo: tt.repo}).UpdateAttemptAnswer(context.Background(), "student-1", 1, tt.questionID, &optionID)
			var handlerErr *platformsocket.HandlerError
			if !errors.As(err, &handlerErr) || handlerErr.Code != tt.code || handlerErr.Message != tt.message {
				t.Fatalf("err = %v, want %s %q", err, tt.code, tt.message)
			}
		})
	}
}

func TestUpdateAttemptAnswerStoresCorrectness(t *testing.T) {
	repo := &fakeAnswerRepository{attempt: answerTestAttempt()}
	optionID := uint(101)
	resp, err := (&service{repo: repo}).UpdateAttemptAnswer(context.Background(), "student-1", 1, 11, &optionID)
	if err != nil || resp.QuestionID != 11 || *resp.SelectedOptionID != 101 {
		t.Fatalf("resp=%#v err=%v", resp, err)
	}
	if len(repo.upserted) != 1 || !*repo.upserted[0].IsCorrect {
		t.Fatalf("upserted = %#v", repo.upserted)
	}
}
