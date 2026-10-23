//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/socket"
)

// UpdateAttemptAnswer (WS ATTEMPT_ANSWER_UPDATED): lưu/xóa đáp án và trả đúng
// sentinel cho từng trường hợp bị từ chối.
func TestUpdateAttemptAnswer(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	svc := newAttemptService(db)

	attempt, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{
		Mode: model.AttemptModePractice, ContextType: model.AttemptContextTypeStandalone, Parts: []model.PartCode{model.Part3},
	})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	part3 := attempt.Sections[0].Groups[0].Questions[0]
	part3Correct, part3Wrong := part3.Options[0].ID, part3.Options[1].ID
	var part2 model.Question
	db.Where("part = ?", model.Part2).First(&part2)

	if _, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, part3.ID, &part3Wrong); err != nil {
		t.Fatal(err)
	}
	resp, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, part3.ID, &part3Correct)
	if err != nil || *resp.SelectedOptionID != part3Correct {
		t.Fatalf("resp=%#v err=%v", resp, err)
	}
	var answers []model.AttemptAnswer
	db.Where("attempt_id = ?", attempt.ID).Find(&answers)
	if len(answers) != 1 || *answers[0].SelectedOptionID != part3Correct || !*answers[0].IsCorrect {
		t.Fatalf("answers after upsert = %#v", answers)
	}
	if _, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, part3.ID, nil); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&model.AttemptAnswer{}).Where("attempt_id = ?", attempt.ID).Count(&count)
	if count != 0 {
		t.Fatalf("clearing the answer must delete it, %d left", count)
	}

	otherOption := part3Correct + 1000
	rejections := []struct {
		name       string
		userID     string
		questionID uint
		optionID   *uint
		want       platformsocket.ErrorCode
	}{
		{"other user's attempt", "student-2", part3.ID, &part3Correct, socket.ErrorCodeAttemptNotFound},
		{"question outside chosen parts", "student-1", part2.ID, nil, socket.ErrorCodeQuestionNotInAttempt},
		{"option of another question", "student-1", part3.ID, &otherOption, socket.ErrorCodeOptionNotInQuestion},
	}
	for _, tt := range rejections {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateAttemptAnswer(ctx, tt.userID, attempt.ID, tt.questionID, tt.optionID)
			assertSocketCode(t, err, tt.want)
		})
	}

	t.Run("expired attempt", func(t *testing.T) {
		db.Model(&model.Attempt{}).Where("id = ?", attempt.ID).Update("expires_at", time.Now().Add(-time.Minute))
		_, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, part3.ID, &part3Correct)
		assertSocketCode(t, err, socket.ErrorCodeAttemptExpired)
	})
	t.Run("attempt not active", func(t *testing.T) {
		db.Model(&model.Attempt{}).Where("id = ?", attempt.ID).Updates(map[string]any{"status": model.AttemptStatusCancelled, "expires_at": nil})
		_, err := svc.UpdateAttemptAnswer(ctx, "student-1", attempt.ID, part3.ID, &part3Correct)
		assertSocketCode(t, err, socket.ErrorCodeAttemptNotActive)
	})
}

func assertSocketCode(t *testing.T, err error, want platformsocket.ErrorCode) {
	t.Helper()
	var handlerErr *platformsocket.HandlerError
	if !errors.As(err, &handlerErr) || handlerErr.Code != want {
		t.Fatalf("err = %v, want WS code %s", err, want)
	}
}
