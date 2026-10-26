package service

import (
	"testing"
	"time"

	"ego/services/learning/internal/model"
)

// flowTestAttempt: 3 trang nghe (audio 10s + gap 5s) rồi 2 trang đọc; đang ở trang nghe `page`.
func flowTestAttempt(page int, startedAt time.Time) *model.Attempt {
	stage := model.AttemptStageListening
	layout := []model.AttemptPage{
		{Index: 0, Section: model.SectionCodeListening, QuestionIDs: []uint{1}, AudioMs: 10000, GapMs: 5000},
		{Index: 1, Section: model.SectionCodeListening, QuestionIDs: []uint{2}, AudioMs: 10000, GapMs: 5000},
		{Index: 2, Section: model.SectionCodeListening, QuestionIDs: []uint{3, 4}, AudioMs: 10000, GapMs: 5000},
		{Index: 3, Section: model.SectionCodeReading, QuestionIDs: []uint{5}},
		{Index: 4, Section: model.SectionCodeReading, QuestionIDs: []uint{6}},
	}
	return &model.Attempt{
		Flow: model.AttemptFlowTOEICStrict, Stage: &stage, Layout: layout,
		ListeningPage: page, ListeningPageStartedAt: &startedAt, StartedAt: startedAt,
	}
}

func TestAdvanceListening(t *testing.T) {
	start := time.Date(2026, 10, 26, 8, 0, 0, 0, time.UTC)
	grace := 20 * time.Second // mỗi trang hết hạn sau 10s + 5s + 20s = 35s

	tests := []struct {
		name        string
		page        int
		after       time.Duration
		wantChanged bool
		wantPage    int
		wantStage   model.AttemptStage
		wantStarted time.Duration
	}{
		{name: "exactly at deadline stays", after: 35 * time.Second, wantPage: 0, wantStage: model.AttemptStageListening},
		{name: "just after deadline advances one page", after: 35*time.Second + time.Millisecond, wantChanged: true, wantPage: 1, wantStage: model.AttemptStageListening, wantStarted: 35 * time.Second},
		{name: "advances several pages", after: 71 * time.Second, wantChanged: true, wantPage: 2, wantStage: model.AttemptStageListening, wantStarted: 70 * time.Second},
		{name: "past last listening page switches to reading", after: 2 * time.Minute, wantChanged: true, wantPage: 3, wantStage: model.AttemptStageReading, wantStarted: 105 * time.Second},
		{name: "last page before deadline", page: 2, after: 30 * time.Second, wantPage: 2, wantStage: model.AttemptStageListening},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := flowTestAttempt(tt.page, start)
			changed := advanceListening(attempt, start.Add(tt.after), grace)
			if changed != tt.wantChanged || attempt.ListeningPage != tt.wantPage || *attempt.Stage != tt.wantStage {
				t.Fatalf("changed=%v page=%d stage=%s, want %v %d %s", changed, attempt.ListeningPage, *attempt.Stage, tt.wantChanged, tt.wantPage, tt.wantStage)
			}
			if tt.wantChanged && !attempt.ListeningPageStartedAt.Equal(start.Add(tt.wantStarted)) {
				t.Fatalf("startedAt = %s, want %s", attempt.ListeningPageStartedAt, start.Add(tt.wantStarted))
			}
		})
	}
}

func TestAdvanceListeningIgnoresFreeAndReading(t *testing.T) {
	start := time.Now()
	free := flowTestAttempt(0, start)
	free.Flow = model.AttemptFlowFree
	reading := flowTestAttempt(3, start)
	stage := model.AttemptStageReading
	reading.Stage = &stage
	for _, attempt := range []*model.Attempt{free, reading} {
		if advanceListening(attempt, start.Add(time.Hour), time.Second) {
			t.Fatalf("attempt %s/%v should not advance", attempt.Flow, *attempt.Stage)
		}
	}
}

func TestPageAndQuestionAccess(t *testing.T) {
	listening := flowTestAttempt(1, time.Now())
	reading := flowTestAttempt(3, time.Now())
	stage := model.AttemptStageReading
	reading.Stage = &stage
	free := flowTestAttempt(0, time.Now())
	free.Flow = model.AttemptFlowFree

	tests := []struct {
		name     string
		attempt  *model.Attempt
		page     int
		question uint
		want     bool
	}{
		{name: "passed listening page locked", attempt: listening, page: 0, question: 1, want: false},
		{name: "current listening page open", attempt: listening, page: 1, question: 2, want: true},
		{name: "future listening page locked", attempt: listening, page: 2, question: 4, want: false},
		{name: "reading locked during listening", attempt: listening, page: 3, question: 5, want: false},
		{name: "reading open after listening", attempt: reading, page: 4, question: 6, want: true},
		{name: "listening locked after listening", attempt: reading, page: 1, question: 2, want: false},
		{name: "free flow opens everything", attempt: free, page: 2, question: 3, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pageAccess(tt.attempt, tt.page); got != tt.want {
				t.Fatalf("pageAccess = %v, want %v", got, tt.want)
			}
			if got := questionAccess(tt.attempt, tt.question); got != tt.want {
				t.Fatalf("questionAccess = %v, want %v", got, tt.want)
			}
		})
	}

	if pageAccess(free, 99) || questionAccess(free, 999) {
		t.Fatal("out-of-range page or foreign question must be denied")
	}
}
