//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	platformsocket "ego/platform/socket"
	"ego/services/learning/config"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/repository"
	"ego/services/learning/internal/service"
	"ego/services/learning/internal/socket"

	"gorm.io/gorm"
)

func requireSocketCode(t *testing.T, err error, code platformsocket.ErrorCode) {
	t.Helper()
	var handlerErr *platformsocket.HandlerError
	if !errors.As(err, &handlerErr) || handlerErr.Code != code {
		t.Fatalf("error = %v, want socket code %s", err, code)
	}
}

// optionIDs trả ID option A (đúng) và B (sai) của câu.
func optionIDs(t *testing.T, db *gorm.DB, questionID uint) (correct, wrong uint) {
	t.Helper()
	var options []model.Option
	if err := db.Where("question_id = ?", questionID).Order("\"order\"").Find(&options).Error; err != nil || len(options) < 2 {
		t.Fatalf("options of %d: %v", questionID, err)
	}
	return options[0].ID, options[1].ID
}

// Cưỡng chế luồng strict ở WS và submit. Layout như TestStrictAttemptPages: trang 0 = Part 2
// (audio 0), trang 1 = group Part 3 (audio 10s), trang 2 = Part 5.
func TestStrictAttemptEnforcement(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	if err := db.Model(&model.Group{}).Where("1 = 1").Update("audio_duration_ms", 10000).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{
		TOEICStrictFlowEnabled: true,
		TOEICListeningGapMs:    [4]int{5000, 5000, 8000, 8000},
	}, nil, testIdentityClient{}, nil, nil, nil)

	created, httpErr := svc.CreateAttempt(ctx, "student-1", exam.ID, dto.CreateExamAttemptRequest{Mode: model.AttemptModeTest, ContextType: model.AttemptContextTypeStandalone})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	var startedAt time.Time
	if err := db.Model(&model.Attempt{}).Select("listening_page_started_at").Where("id = ?", created.ID).Scan(&startedAt).Error; err != nil {
		t.Fatal(err)
	}
	now := startedAt
	service.SetClock(svc, func() time.Time { return now })

	nav, httpErr := svc.GetAttemptNavigator(ctx, "student-1", created.ID)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	part2 := nav.Pages[0].Questions[0].ID
	part3 := nav.Pages[1].Questions[0].ID
	part5 := nav.Pages[2].Questions[0].ID
	part2Correct, part2Wrong := optionIDs(t, db, part2)
	part3Correct, _ := optionIDs(t, db, part3)
	part5Correct, _ := optionIDs(t, db, part5)

	// Trang 0 đang mở: chọn được; câu trang sau và trang đọc bị khóa.
	if _, err := svc.UpdateAttemptAnswer(ctx, "student-1", created.ID, part2, &part2Correct); err != nil {
		t.Fatal(err)
	}
	_, err := svc.UpdateAttemptAnswer(ctx, "student-1", created.ID, part3, &part3Correct)
	requireSocketCode(t, err, socket.ErrorCodeQuestionLocked)
	_, err = svc.UpdateAttemptAnswer(ctx, "student-1", created.ID, part5, &part5Correct)
	requireSocketCode(t, err, socket.ErrorCodeQuestionLocked)

	// Advance lệch trang bị từ chối; đúng trang (audio 0) được nhận, startedAt = now.
	_, err = svc.AdvanceListening(ctx, "student-1", created.ID, 1)
	requireSocketCode(t, err, socket.ErrorCodeListeningPageMismatch)
	now = startedAt.Add(4 * time.Second)
	state, err := svc.AdvanceListening(ctx, "student-1", created.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Stage != "LISTENING" || state.CurrentPage == nil || *state.CurrentPage != 1 || !state.PageStartedAt.Equal(now) {
		t.Fatalf("state = %#v", state)
	}
	pageOneStart := now

	// Trang 1 có audio 10s: advance trước 8s là quá sớm.
	now = pageOneStart.Add(7 * time.Second)
	_, err = svc.AdvanceListening(ctx, "student-1", created.ID, 1)
	requireSocketCode(t, err, socket.ErrorCodeListeningAdvanceTooEarly)

	// F5 giữa trang 1: elapsedMs tính từ lúc server nhận advance.
	page, httpErr := svc.GetAttemptPage(ctx, "student-1", created.ID, 1)
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if page.Listening.ElapsedMs != 7000 {
		t.Fatalf("elapsedMs = %d, want 7000", page.Listening.ElapsedMs)
	}

	// Câu trang 0 giờ đã khóa.
	_, err = svc.UpdateAttemptAnswer(ctx, "student-1", created.ID, part2, &part2Wrong)
	requireSocketCode(t, err, socket.ErrorCodeQuestionLocked)

	// Nộp khi đang nghe trang 1: body đổi câu đã khóa và trả lời trang đọc đều bị bỏ qua.
	result, httpErr := svc.SubmitAttempt(ctx, "student-1", created.ID, dto.SubmitAttemptRequest{Answers: []dto.SubmitAttemptAnswer{
		{QuestionID: part2, SelectedOptionID: &part2Wrong},
		{QuestionID: part3, SelectedOptionID: &part3Correct},
		{QuestionID: part5, SelectedOptionID: &part5Correct},
	}})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if result.CorrectAnswers == nil || *result.CorrectAnswers != 2 {
		t.Fatalf("correctAnswers = %v, want 2 (saved part 2 + body part 3)", result.CorrectAnswers)
	}
	var saved []model.AttemptAnswer
	if err := db.Where("attempt_id = ?", created.ID).Find(&saved).Error; err != nil {
		t.Fatal(err)
	}
	for _, answer := range saved {
		if answer.QuestionID == part5 {
			t.Fatal("reading answer submitted during listening must be ignored")
		}
		if answer.QuestionID == part2 && *answer.SelectedOptionID != part2Correct {
			t.Fatal("locked question must keep its saved answer")
		}
	}
}

// Luyện chỉ phần nghe (Part 2 + 3): nghe xong trang cuối thì server nộp bài như hết giờ, qua
// cả ba đường: client gửi advance, lazy advance khi có request, và worker khi đóng tab.
func TestListeningOnlyAttemptAutoSubmit(t *testing.T) {
	db := learningTestDB(t)
	ctx := context.Background()
	exam := seedTOEIC(t, db)
	if err := db.Model(&model.Group{}).Where("1 = 1").Update("audio_duration_ms", 10000).Error; err != nil {
		t.Fatal(err)
	}
	svc := service.New(repository.NewRepository(db), &config.AppConfig{
		TOEICStrictFlowEnabled: true,
		TOEICListeningGapMs:    [4]int{5000, 5000, 8000, 8000},
	}, nil, testIdentityClient{}, nil, nil, nil)

	start := func(userID string) (uint, time.Time) {
		t.Helper()
		created, httpErr := svc.CreateAttempt(ctx, userID, exam.ID, dto.CreateExamAttemptRequest{
			Mode: model.AttemptModePractice, Parts: []model.PartCode{model.Part2, model.Part3},
			ContextType: model.AttemptContextTypeStandalone,
		})
		if httpErr != nil {
			t.Fatal(httpErr)
		}
		var startedAt time.Time
		if err := db.Model(&model.Attempt{}).Select("listening_page_started_at").Where("id = ?", created.ID).Scan(&startedAt).Error; err != nil {
			t.Fatal(err)
		}
		return created.ID, startedAt
	}
	requireStatus := func(attemptID uint, want model.AttemptStatus) {
		t.Helper()
		var status model.AttemptStatus
		if err := db.Model(&model.Attempt{}).Select("status").Where("id = ?", attemptID).Scan(&status).Error; err != nil {
			t.Fatal(err)
		}
		if status != want {
			t.Fatalf("attempt %d status = %s, want %s", attemptID, status, want)
		}
	}

	t.Run("advance on last page submits", func(t *testing.T) {
		id, startedAt := start("listen-advance")
		now := startedAt
		service.SetClock(svc, func() time.Time { return now })
		if _, err := svc.AdvanceListening(ctx, "listen-advance", id, 0); err != nil {
			t.Fatal(err)
		}
		requireStatus(id, model.AttemptStatusInProgress)
		now = now.Add(10 * time.Second)
		state, err := svc.AdvanceListening(ctx, "listen-advance", id, 1)
		if err != nil {
			t.Fatal(err)
		}
		if state.Stage != string(model.AttemptStageReading) {
			t.Fatalf("stage = %s, want READING", state.Stage)
		}
		requireStatus(id, model.AttemptStatusSubmitted)
	})

	t.Run("lazy advance on request submits", func(t *testing.T) {
		id, startedAt := start("listen-lazy")
		service.SetClock(svc, func() time.Time { return startedAt.Add(5 * time.Minute) })
		_, _ = svc.GetAttemptNavigator(ctx, "listen-lazy", id)
		requireStatus(id, model.AttemptStatusSubmitted)
	})

	t.Run("worker submits after tab closed", func(t *testing.T) {
		id, startedAt := start("listen-worker")
		service.SetClock(svc, func() time.Time { return startedAt.Add(time.Second) })
		if _, err := svc.AutoSubmitExpiredAttempts(ctx, 10); err != nil {
			t.Fatal(err)
		}
		requireStatus(id, model.AttemptStatusInProgress)
		service.SetClock(svc, func() time.Time { return startedAt.Add(5 * time.Minute) })
		if _, err := svc.AutoSubmitExpiredAttempts(ctx, 10); err != nil {
			t.Fatal(err)
		}
		requireStatus(id, model.AttemptStatusSubmitted)
	})
}
