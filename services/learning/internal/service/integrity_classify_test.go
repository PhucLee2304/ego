package service

import (
	"testing"
	"time"

	"ego/services/learning/internal/model"

	"gorm.io/datatypes"
)

func TestClassifyEpisode(t *testing.T) {
	base := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	conn := "c1"
	other := "c2"
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	ptr := func(v time.Time) *time.Time { return &v }

	tests := []struct {
		name  string
		start model.AttemptIntegrityEvent
		end   model.AttemptIntegrityEvent
		want  model.IntegrityLevel
	}{
		{"rời 10s ngoài ân hạn", model.AttemptIntegrityEvent{ReceivedAt: at(0)}, model.AttemptIntegrityEvent{ReceivedAt: at(10000)}, model.IntegrityLevelViolation},
		{"đúng biên 3s", model.AttemptIntegrityEvent{ReceivedAt: at(0)}, model.AttemptIntegrityEvent{ReceivedAt: at(3000)}, model.IntegrityLevelViolation},
		{"ngắn hơn 3s", model.AttemptIntegrityEvent{ReceivedAt: at(0)}, model.AttemptIntegrityEvent{ReceivedAt: at(2999)}, model.IntegrityLevelLog},
		{"nằm gọn trong ân hạn", model.AttemptIntegrityEvent{ReceivedAt: at(0), GraceUntil: ptr(at(10000))}, model.AttemptIntegrityEvent{ReceivedAt: at(8000)}, model.IntegrityLevelWarning},
		{"bắt đầu giây 8 của ân hạn, kéo dài 4s", model.AttemptIntegrityEvent{ReceivedAt: at(8000), GraceUntil: ptr(at(10000))}, model.AttemptIntegrityEvent{ReceivedAt: at(12000)}, model.IntegrityLevelWarning},
		{"F5 rồi rời ngay 2 phút", model.AttemptIntegrityEvent{ReceivedAt: at(500), GraceUntil: ptr(at(10000))}, model.AttemptIntegrityEvent{ReceivedAt: at(120500)}, model.IntegrityLevelViolation},
		{"ân hạn đã hết trước khi rời", model.AttemptIntegrityEvent{ReceivedAt: at(20000), GraceUntil: ptr(at(10000))}, model.AttemptIntegrityEvent{ReceivedAt: at(24000)}, model.IntegrityLevelViolation},
		{"gửi bù", model.AttemptIntegrityEvent{ReceivedAt: at(0)}, model.AttemptIntegrityEvent{ReceivedAt: at(60000), Replayed: true}, model.IntegrityLevelWarning},
		{"mất kết nối giữa chừng", model.AttemptIntegrityEvent{ReceivedAt: at(0)}, model.AttemptIntegrityEvent{ReceivedAt: at(60000), ConnectionID: &other}, model.IntegrityLevelWarning},
		{"stale", model.AttemptIntegrityEvent{ReceivedAt: at(0), Stale: true}, model.AttemptIntegrityEvent{ReceivedAt: at(60000)}, model.IntegrityLevelLog},
		{"khác epoch", model.AttemptIntegrityEvent{ReceivedAt: at(0), SessionEpoch: 1}, model.AttemptIntegrityEvent{ReceivedAt: at(60000), SessionEpoch: 1}, model.IntegrityLevelLog},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := &model.Attempt{SessionEpoch: 2}
			for _, ev := range []*model.AttemptIntegrityEvent{&tt.start, &tt.end} {
				if ev.SessionEpoch == 0 {
					ev.SessionEpoch = 2
				}
				if ev.ConnectionID == nil {
					ev.ConnectionID = &conn
				}
			}
			if got := classifyEpisode(&tt.start, &tt.end, attempt, defaultIntegrityConfig); got != tt.want {
				t.Fatalf("classifyEpisode = %s, want %s", got, tt.want)
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
