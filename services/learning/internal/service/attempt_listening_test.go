package service

import (
	"testing"
	"time"

	platformsocket "ego/platform/socket"
	"ego/services/learning/internal/dto"
	"ego/services/learning/internal/model"
	"ego/services/learning/internal/socket"
)

func TestAcceptListeningAdvance(t *testing.T) {
	start := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	// audio 10s → nhận advance từ start + 8s.
	tests := []struct {
		name      string
		current   int
		page      int
		after     time.Duration
		accepted  bool
		code      platformsocket.ErrorCode
		wantPage  int
		wantStage model.AttemptStage
	}{
		{name: "too early", page: 0, after: 7999 * time.Millisecond, code: socket.ErrorCodeListeningAdvanceTooEarly, wantStage: model.AttemptStageListening},
		{name: "within 2s tolerance", page: 0, after: 8 * time.Second, accepted: true, wantPage: 1, wantStage: model.AttemptStageListening},
		{name: "page mismatch", page: 1, after: time.Minute, code: socket.ErrorCodeListeningPageMismatch, wantStage: model.AttemptStageListening},
		{name: "last listening page switches to reading", current: 2, page: 2, after: 10 * time.Second, accepted: true, wantPage: 3, wantStage: model.AttemptStageReading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := flowTestAttempt(tt.current, start)
			now := start.Add(tt.after)
			accepted, code := acceptListeningAdvance(attempt, tt.page, now)
			if accepted != tt.accepted || code != tt.code {
				t.Fatalf("accepted=%v code=%s, want %v %s", accepted, code, tt.accepted, tt.code)
			}
			if *attempt.Stage != tt.wantStage {
				t.Fatalf("stage = %s, want %s", *attempt.Stage, tt.wantStage)
			}
			if accepted && (attempt.ListeningPage != tt.wantPage || !attempt.ListeningPageStartedAt.Equal(now)) {
				t.Fatalf("page=%d startedAt=%s, want %d %s", attempt.ListeningPage, attempt.ListeningPageStartedAt, tt.wantPage, now)
			}
		})
	}

	reading := flowTestAttempt(3, start)
	stage := model.AttemptStageReading
	reading.Stage = &stage
	if accepted, code := acceptListeningAdvance(reading, 3, start.Add(time.Hour)); accepted || code != socket.ErrorCodeListeningPageMismatch {
		t.Fatal("advance in READING must be a mismatch")
	}
}

func TestStrictSubmittedAnswers(t *testing.T) {
	option := func(v uint) *uint { return &v }
	// Đang nghe trang 1 (câu 2); câu 1 đã qua, đã lưu đáp án 100.
	attempt := flowTestAttempt(1, time.Now())
	attempt.Answers = []model.AttemptAnswer{
		{QuestionID: 1, SelectedOptionID: option(100)},
		{QuestionID: 2, SelectedOptionID: option(200)},
	}

	got := strictSubmittedAnswers(attempt, []dto.SubmitAttemptAnswer{
		{QuestionID: 1, SelectedOptionID: option(101)}, // khóa → bỏ body, giữ 100
		{QuestionID: 2, SelectedOptionID: option(201)}, // đang mở → lấy body
		{QuestionID: 5, SelectedOptionID: option(500)}, // trang đọc khi đang nghe → bỏ
		{QuestionID: 999, SelectedOptionID: option(9)}, // không thuộc attempt → giữ để báo lỗi
	})

	want := map[uint]uint{2: 201, 999: 9, 1: 100}
	if len(got) != len(want) {
		t.Fatalf("answers = %+v", got)
	}
	for _, answer := range got {
		if want[answer.QuestionID] != *answer.SelectedOptionID {
			t.Fatalf("question %d = %d, want %d", answer.QuestionID, *answer.SelectedOptionID, want[answer.QuestionID])
		}
	}

	// Câu đang mở mà body không nhắc tới giữ đáp án đã lưu.
	got = strictSubmittedAnswers(attempt, nil)
	want = map[uint]uint{1: 100, 2: 200}
	if len(got) != len(want) {
		t.Fatalf("answers without body = %+v", got)
	}
	for _, answer := range got {
		if want[answer.QuestionID] != *answer.SelectedOptionID {
			t.Fatalf("question %d = %d, want %d", answer.QuestionID, *answer.SelectedOptionID, want[answer.QuestionID])
		}
	}
}
