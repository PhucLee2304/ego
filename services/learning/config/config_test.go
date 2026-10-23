package config

import (
	"testing"
	"time"
)

func TestGetDurationEnv(t *testing.T) {
	const fallback = 5 * time.Minute

	t.Setenv("TEST_DURATION", "90s")
	if got := getDurationEnv("TEST_DURATION", fallback); got != 90*time.Second {
		t.Fatalf("getDurationEnv() = %s, want 90s", got)
	}

	for _, value := range []string{"", "invalid", "0s", "-1m"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("TEST_DURATION", value)
			if got := getDurationEnv("TEST_DURATION", fallback); got != fallback {
				t.Fatalf("getDurationEnv() = %s, want fallback %s", got, fallback)
			}
		})
	}
}

func TestLoadAppConfigRequiresLiveKitSecrets(t *testing.T) {
	tests := []struct {
		name, key, secret string
		wantErr           bool
	}{
		{name: "thiếu cả hai", wantErr: true},
		{name: "thiếu secret", key: "k", wantErr: true},
		{name: "đủ", key: "k", secret: "s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LIVEKIT_API_KEY", tt.key)
			t.Setenv("LIVEKIT_API_SECRET", tt.secret)
			_, err := LoadAppConfig()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadAppConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
