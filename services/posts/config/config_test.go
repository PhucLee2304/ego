package config

import (
	"strings"
	"testing"
)

func TestLoadAppConfig(t *testing.T) {
	t.Setenv("APP_PORT", "9090")
	t.Setenv("DB_HOST", "database")
	t.Setenv("DB_PASSWORD", "secret")

	appConfig, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() returned an error: %v", err)
	}

	if appConfig.Port != "9090" {
		t.Fatalf("expected port 9090, got %q", appConfig.Port)
	}
	if appConfig.DBHost != "database" {
		t.Fatalf("expected database host, got %q", appConfig.DBHost)
	}
	if !strings.Contains(appConfig.DSN, "host=database") || !strings.Contains(appConfig.DSN, "password=secret") {
		t.Fatalf("unexpected DSN: %q", appConfig.DSN)
	}
}
