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
