package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadAppConfig(t *testing.T) {
	for key, value := range map[string]string{
		"APP_PORT": "9090", "APP_MODE": "development",
		"DB_HOST": "database", "DB_PORT": "5432", "DB_USER": "postgres", "DB_PASSWORD": "secret", "DB_NAME": "postsdb",
		"IDENTITY_SERVICE_ADDR": "identity:50052", "LEARNING_SERVICE_ADDR": "learning:50059",
		"NOTIFICATIONS_SERVICE_ADDR": "notifications:50058", "STORAGE_SERVICE_ADDR": "storage:50054",
		"POST_IMAGE_MAX_BYTES": "5242880", "POST_FILE_MAX_BYTES": "10485760", "POST_VIDEO_MAX_BYTES": "20971520",
		"POST_TOTAL_MAX_BYTES": "52428800", "POST_MAX_ATTACHMENTS": "10", "POST_UPLOAD_TTL_MINUTES": "60",
	} {
		t.Setenv(key, value)
	}

	appConfig := LoadAppConfig()

	if appConfig.Port != "9090" || appConfig.UploadIntentTTL != time.Hour || appConfig.MaxAttachments != 10 {
		t.Fatalf("unexpected config: %+v", appConfig)
	}
	if !strings.Contains(appConfig.DSN, "host=database") || !strings.Contains(appConfig.DSN, "password=secret") {
		t.Fatalf("unexpected DSN: %q", appConfig.DSN)
	}
}
