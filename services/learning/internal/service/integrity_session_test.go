package service

import (
	"testing"
	"time"

	"ego/services/learning/internal/model"
)

func TestDecideSessionClaim(t *testing.T) {
	now := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	token := "secret"
	hash := hashSessionToken(token)
	seen := func(ago time.Duration) *time.Time { v := now.Add(-ago); return &v }

	tests := []struct {
		name      string
		hash      *string
		lastSeen  *time.Time
		liveOwner string
		token     string
		takeover  bool
		want      claimResult
		accepted  bool
		epoch     int
		conflict  bool
	}{
		{"claim đầu tiên", nil, nil, "", "", false, claimFirst, true, 4, false},
		{"token đúng, không ai giữ (F5)", &hash, seen(time.Second), "", token, false, claimResume, true, 3, false},
		{"token đúng, chính kết nối này", &hash, nil, "me", token, false, claimResume, true, 3, false},
		{"token đúng, kết nối khác đang giữ", &hash, nil, "other", token, false, claimRejectInUse, false, 0, true},
		{"không token, chủ đang sống", &hash, nil, "other", "", false, claimRejectInUse, false, 0, true},
		{"token sai, chủ đang sống", &hash, nil, "other", "wrong", true, claimRejectInUse, false, 0, true},
		{"chủ ngắt chưa quá 30s", &hash, seen(10 * time.Second), "", "", true, claimRejectRecovering, false, 0, false},
		{"chủ ngắt quá 30s, có takeover", &hash, seen(31 * time.Second), "", "", true, claimTakeover, true, 4, false},
		{"chủ ngắt quá 30s, chưa xác nhận", &hash, seen(31 * time.Second), "", "", false, claimRejectTakeoverRequired, false, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempt := &model.Attempt{SessionTokenHash: tt.hash, SessionEpoch: 3, SessionLastSeenAt: tt.lastSeen}
			got := decideSessionClaim(attempt, tt.liveOwner, "me", tt.token, tt.takeover, now, defaultIntegrityConfig)
			if got.Result != tt.want || got.Accepted != tt.accepted || got.Conflict != tt.conflict {
				t.Fatalf("decideSessionClaim = %+v, want %s accepted=%v conflict=%v", got, tt.want, tt.accepted, tt.conflict)
			}
			if tt.accepted && got.Epoch != tt.epoch {
				t.Fatalf("epoch = %d, want %d", got.Epoch, tt.epoch)
			}
			if tt.want == claimRejectRecovering && got.RetryAfter != 20*time.Second {
				t.Fatalf("retryAfter = %s, want 20s", got.RetryAfter)
			}
		})
	}
}

func TestNewSessionToken(t *testing.T) {
	token, hash, err := newSessionToken()
	if err != nil {
		t.Fatal(err)
	}
	if !sessionTokenMatches(hash, token) || sessionTokenMatches(hash, token+"x") || sessionTokenMatches(hash, "") {
		t.Fatal("sessionTokenMatches sai")
	}
}
