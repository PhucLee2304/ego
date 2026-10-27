package service

import (
	"testing"

	"ego/services/learning/internal/model"
)

func TestApplyScore(t *testing.T) {
	tests := []struct {
		name    string
		outcome model.IntegrityOutcome
		want    float64
	}{
		{"bình thường", model.IntegrityOutcomeNone, 7.5},
		{"đình chỉ", model.IntegrityOutcomeTerminated, 0},
		{"đã khôi phục", model.IntegrityOutcomeReinstated, 7.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := &model.Attempt{IntegrityOutcome: tt.outcome}
			applyScore(attempt, 30, 7.5)
			if *attempt.Score != tt.want || *attempt.RawScore != 7.5 || *attempt.CorrectAnswers != 30 {
				t.Fatalf("score=%v raw=%v correct=%v", *attempt.Score, *attempt.RawScore, *attempt.CorrectAnswers)
			}
		})
	}
}
