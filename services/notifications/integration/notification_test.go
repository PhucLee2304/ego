//go:build integration

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/notifications/database"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/model"
	"ego/services/notifications/internal/repository"
	"ego/services/notifications/internal/service"
	notificationsRpc "ego/services/notifications/rpc"

	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type fakeSender struct {
	called  chan struct{}
	invalid []string
}

func (f *fakeSender) Send(context.Context, []string, string, string, map[string]string) ([]string, error) {
	close(f.called)
	return f.invalid, nil
}

func testRepository(t *testing.T) (*repository.Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(os.Getenv("NOTIFICATIONS_TEST_DSN")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.DeviceToken{}, &model.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return repository.New(db), db
}

func createNotifications(t *testing.T, repo *repository.Repository) {
	t.Helper()
	items := []*model.Notification{
		{UserID: "1", Title: "Newest", Body: "Body", SourceService: "TEST", Data: datatypes.JSONMap{"id": "3", "type": "ROOM"}, IdempotencyKey: "n-3"},
		{UserID: "1", Title: "Second", Body: "Body", SourceService: "TEST", Data: datatypes.JSONMap{"id": "2", "type": "CLASSROOM"}, IdempotencyKey: "n-2"},
		{UserID: "2", Title: "Other user", Body: "Body", SourceService: "TEST", Data: datatypes.JSONMap{}, IdempotencyKey: "n-other"},
	}
	if err := repo.CreateNotifications(context.Background(), items); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationListReadAndDelete(t *testing.T) {
	repo, db := testRepository(t)
	createNotifications(t, repo)
	ctx := context.Background()

	items, total, unread, err := repo.GetNotifications(ctx, "1", nil, nil, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || total != 2 || unread != 2 || items[0].Title != "Second" {
		t.Fatalf("unexpected initial list: len=%d total=%d unread=%d title=%q", len(items), total, unread, items[0].Title)
	}

	firstID := items[0].ID
	if err := repo.MarkRead(ctx, "1", []uint{firstID}); err != nil {
		t.Fatal(err)
	}
	read := true
	items, total, unread, err = repo.GetNotifications(ctx, "1", &read, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || total != 1 || unread != 1 {
		t.Fatalf("unexpected read filter: len=%d total=%d unread=%d", len(items), total, unread)
	}

	if err := repo.MarkRead(ctx, "1", nil); err != nil {
		t.Fatal(err)
	}
	unreadOnly := false
	items, total, unread, err = repo.GetNotifications(ctx, "1", &unreadOnly, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || total != 0 || unread != 0 {
		t.Fatalf("read all failed: len=%d total=%d unread=%d", len(items), total, unread)
	}

	var other model.Notification
	if err := db.Where("user_id = ?", "2").First(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteNotifications(ctx, "1", []uint{firstID, other.ID}); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&other, other.ID).Error; err != nil {
		t.Fatalf("deleted another user's notification: %v", err)
	}
}

func TestDeviceTokenUpsertAndScopedDelete(t *testing.T) {
	repo, db := testRepository(t)
	ctx := context.Background()
	if err := repo.UpsertDeviceToken(ctx, &model.DeviceToken{UserID: "1", DeviceID: "browser-a", Token: "token-a", Platform: model.TokenPlatformWeb}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertDeviceToken(ctx, &model.DeviceToken{UserID: "1", DeviceID: "browser-b", Token: "token-b", Platform: model.TokenPlatformWeb}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.DeviceToken{}).Where("user_id = ? AND platform = ?", "1", model.TokenPlatformWeb).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("multiple web installations were not preserved: %d", count)
	}
	if err := repo.UpsertDeviceToken(ctx, &model.DeviceToken{UserID: "2", DeviceID: "browser-c", Token: "token-b", Platform: model.TokenPlatformWeb}); err == nil {
		t.Fatal("expected a token assigned to another installation to be rejected")
	}
	var original model.DeviceToken
	if err := db.Where("device_id = ?", "browser-b").First(&original).Error; err != nil {
		t.Fatal(err)
	}
	if original.UserID != "1" || original.Token != "token-b" {
		t.Fatalf("conflicting registration changed the original installation: %+v", original)
	}
	if err := repo.UpsertDeviceToken(ctx, &model.DeviceToken{UserID: "2", DeviceID: "browser-a", Token: "token-c", Platform: model.TokenPlatformWeb}); err != nil {
		t.Fatal(err)
	}
	var saved model.DeviceToken
	if err := db.Where("device_id = ?", "browser-a").First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if saved.UserID != "2" || saved.Token != "token-c" {
		t.Fatalf("installation was not transferred to the current user: %+v", saved)
	}
	if err := repo.DeleteDeviceToken(ctx, "1", "token-c"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("token = ?", "token-c").First(&model.DeviceToken{}).Error; err != nil {
		t.Fatalf("wrong user deleted token: %v", err)
	}
	if err := repo.DeleteDeviceToken(ctx, "2", "token-c"); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("token = ?", "token-c").First(&model.DeviceToken{}).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("token was not hard deleted: %v", err)
	}
}

func TestCreateNotificationCommitsBeforeBestEffortPush(t *testing.T) {
	repo, db := testRepository(t)
	ctx := context.Background()
	if err := repo.UpsertDeviceToken(ctx, &model.DeviceToken{UserID: "1", DeviceID: "browser", Token: "invalid-token", Platform: model.TokenPlatformWeb}); err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{called: make(chan struct{}), invalid: []string{"invalid-token"}}
	svc := service.New(repo, sender, nil)
	if err := svc.CreateNotifications(ctx, []dto.CreateNotificationRequest{{
		UserID: "1", Title: "Saved first", Body: "Push is optional", SourceService: "TEST",
		IdempotencyKey: "saved-before-push", Data: map[string]string{"id": "1", "type": "HOME"},
	}}); err != nil {
		t.Fatal(err)
	}

	var count int64
	if err := db.Model(&model.Notification{}).Where("idempotency_key = ?", "saved-before-push").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("notification was not committed: count=%d err=%v", count, err)
	}
	select {
	case <-sender.called:
	case <-time.After(2 * time.Second):
		t.Fatal("push sender was not called")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var tokenCount int64
		if err := db.Model(&model.DeviceToken{}).Where("token = ?", "invalid-token").Count(&tokenCount).Error; err != nil {
			t.Fatal(err)
		}
		if tokenCount == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("invalid token was not deleted")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCreateNotificationsIsIdempotent(t *testing.T) {
	repo, db := testRepository(t)
	ctx := context.Background()
	item := &model.Notification{UserID: "1", Title: "Once", Body: "Body", SourceService: "CLASSROOMS", IdempotencyKey: "assignment:1:user:1"}
	if err := repo.CreateNotifications(ctx, []*model.Notification{item}); err != nil {
		t.Fatal(err)
	}
	duplicate := &model.Notification{UserID: "1", Title: "Duplicate", Body: "Body", SourceService: "CLASSROOMS", IdempotencyKey: item.IdempotencyKey}
	if err := repo.CreateNotifications(ctx, []*model.Notification{duplicate}); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Notification{}).Where("idempotency_key = ?", item.IdempotencyKey).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one notification, got %d", count)
	}
}

func TestCreateNotificationsRPCCommitsAndDeduplicates(t *testing.T) {
	repo, db := testRepository(t)
	server := notificationsRpc.New(service.New(repo, nil, nil))
	request := &notificationsClient.CreateNotificationsRequest{
		Recipients: []string{"1", "2"}, Title: "Assignment", Body: "A new assignment is available",
		SourceService: "CLASSROOMS", Data: map[string]string{"id": "9", "type": "ASSIGNMENT"}, IdempotencyKey: "assignment:9:created",
	}
	if _, err := server.CreateNotifications(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := server.CreateNotifications(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Notification{}).Where("idempotency_key LIKE ?", "assignment:9:created:user:%").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected one committed notification per recipient, got %d", count)
	}
}
