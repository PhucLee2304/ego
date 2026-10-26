package main

import (
	"context"
	"os/exec"
	"testing"
)

func TestProbeAudioMeasuresDurationAndTrailingSilence(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	// 1s sine + 1.5s silence.
	info, err := probeAudio(context.Background(), "testdata/tone_1s_silence_1500ms.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs < 2400 || info.DurationMs > 2700 {
		t.Fatalf("DurationMs = %d, want ~2500", info.DurationMs)
	}
	if info.TrailingSilenceMs < 1300 || info.TrailingSilenceMs > 1700 {
		t.Fatalf("TrailingSilenceMs = %d, want ~1500", info.TrailingSilenceMs)
	}
}

func TestTrailingSilence(t *testing.T) {
	tests := []struct {
		name     string
		log      string
		duration float64
		want     float64
	}{
		{name: "no silence", log: "", duration: 10, want: 0},
		{name: "silence without end", log: "silence_start: 7.5", duration: 10, want: 2.5},
		{name: "silence end at eof", log: "silence_start: 2\nsilence_end: 3\nsilence_start: 8\nsilence_end: 10", duration: 10, want: 2},
		{name: "last silence in the middle", log: "silence_start: 2\nsilence_end: 3", duration: 10, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trailingSilence(tt.log, tt.duration); got != tt.want {
				t.Fatalf("trailingSilence = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToeicQuestionNumber(t *testing.T) {
	tests := []struct {
		name       string
		groupOrder int
		questionNo int
		want       *int
	}{
		{name: "part 1 single", groupOrder: 1, questionNo: 1, want: intPtr(1)},
		{name: "part 3 third in group", groupOrder: 32, questionNo: 3, want: intPtr(34)},
		{name: "part 7 fifth in group", groupOrder: 196, questionNo: 5, want: intPtr(200)},
		{name: "missing order", groupOrder: 0, questionNo: 1, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toeicQuestionNumber(toeicQuestionRow{GroupOrder: tt.groupOrder, QuestionNo: tt.questionNo})
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Fatalf("toeicQuestionNumber = %v, want %v", got, tt.want)
			}
		})
	}
}

func intPtr(v int) *int { return &v }
