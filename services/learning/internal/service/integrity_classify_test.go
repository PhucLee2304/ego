package service

import (
	"testing"

	"ego/services/learning/internal/model"

	"gorm.io/datatypes"
)

func TestClassifyAwayStart(t *testing.T) {
	tests := []struct {
		name  string
		start model.AttemptIntegrityEvent
		want  model.IntegrityLevel
	}{
		{"rời màn từ chủ phiên", model.AttemptIntegrityEvent{SessionEpoch: 2}, model.IntegrityLevelViolation},
		{"gửi bù", model.AttemptIntegrityEvent{SessionEpoch: 2, Replayed: true}, model.IntegrityLevelWarning},
		{"stale", model.AttemptIntegrityEvent{SessionEpoch: 2, Stale: true}, model.IntegrityLevelLog},
		{"khác epoch", model.AttemptIntegrityEvent{SessionEpoch: 1}, model.IntegrityLevelLog},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyAwayStart(&tt.start, &model.Attempt{SessionEpoch: 2}); got != tt.want {
				t.Fatalf("classifyAwayStart = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestExceededMaxViolations(t *testing.T) {
	tests := []struct {
		name      string
		proctored bool
		max       int
		count     int
		want      bool
	}{
		{"Max 1, lần 1", true, 1, 1, false},
		{"Max 1, lần 2 văng", true, 1, 2, true},
		{"Max 2, lần 2", true, 2, 2, false},
		{"Max 2, lần 3 văng", true, 2, 3, true},
		{"không giám sát", false, 1, 5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := datatypes.NewJSONType(model.IntegrityPolicySnapshot{MaxViolations: tt.max})
			attempt := &model.Attempt{Proctored: tt.proctored, IntegrityPolicy: &policy, ViolationCount: tt.count}
			if got := exceededMaxViolations(attempt); got != tt.want {
				t.Fatalf("exceededMaxViolations = %v, want %v", got, tt.want)
			}
		})
	}
}
