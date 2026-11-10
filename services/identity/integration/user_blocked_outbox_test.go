//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/service"

	"google.golang.org/grpc"
)

type outboxNotificationsClient struct {
	calls int
	err   error
}

func (f *outboxNotificationsClient) DeleteDeviceTokensByUserID(context.Context, *notificationsClient.DeleteDeviceTokensByUserIDRequest, ...grpc.CallOption) (*notificationsClient.DeleteDeviceTokensByUserIDResponse, error) {
	f.calls++
	return &notificationsClient.DeleteDeviceTokensByUserIDResponse{}, f.err
}

type outboxLearningClient struct{ calls int }

func (f *outboxLearningClient) RemoveUserFromLiveRooms(context.Context, *learningClient.RemoveUserFromLiveRoomsRequest, ...grpc.CallOption) (*learningClient.RemoveUserFromLiveRoomsResponse, error) {
	f.calls++
	return &learningClient.RemoveUserFromLiveRoomsResponse{}, nil
}

// TestUserBlockedOutbox: khoá ghi event cùng transaction; rpc lỗi thì event được giữ lại và hẹn thử lại;
// lần sau thành công thì đánh dấu xong; mở khoá huỷ event còn chờ.
func TestUserBlockedOutbox(t *testing.T) {
	_, db := testService(t)
	ctx := context.Background()
	userID := createUser(t, db, "blocked@example.com", "Blocked", model.RoleClient)
	notifications := &outboxNotificationsClient{err: errors.New("unavailable")}
	learning := &outboxLearningClient{}
	svc := service.New(nil, nil, repository.NewRepository(db), nil, notifications, learning)

	if _, httpErr := svc.SetBlocked(ctx, userID, true); httpErr != nil {
		t.Fatal(httpErr)
	}
	var event model.UserBlockedOutbox
	if err := db.Where("user_id = ?", userID).First(&event).Error; err != nil || event.ProcessedAt != nil {
		t.Fatalf("event=%+v err=%v, want một event chưa xử lý", event, err)
	}

	// Lần 1: notifications lỗi, event vẫn chờ, attempts tăng và next_attempt_at lùi về sau.
	if processed, err := svc.ProcessUserBlockedOutbox(ctx, 10); err != nil || processed != 0 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	db.First(&event, event.ID)
	if event.ProcessedAt != nil || event.Attempts != 1 || event.LastError == nil || !event.NextAttemptAt.After(time.Now()) {
		t.Fatalf("sau lần lỗi: %+v", event)
	}
	// Chưa tới hạn thì không bị claim lại.
	if processed, _ := svc.ProcessUserBlockedOutbox(ctx, 10); processed != 0 || notifications.calls != 1 {
		t.Fatalf("event chưa tới hạn bị xử lý lại: processed=%d calls=%d", processed, notifications.calls)
	}

	// Lần 2: tới hạn và rpc thành công thì đánh dấu xong.
	db.Model(&event).Update("next_attempt_at", time.Now().Add(-time.Second))
	notifications.err = nil
	if processed, err := svc.ProcessUserBlockedOutbox(ctx, 10); err != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	db.First(&event, event.ID)
	if event.ProcessedAt == nil || event.LastError != nil || learning.calls != 2 {
		t.Fatalf("sau lần thành công: %+v learning.calls=%d", event, learning.calls)
	}

	// Mở khoá rồi khoá lại, rồi mở khoá trước khi worker chạy: event mới bị huỷ, worker không gọi rpc.
	if _, httpErr := svc.SetBlocked(ctx, userID, false); httpErr != nil {
		t.Fatal(httpErr)
	}
	if _, httpErr := svc.SetBlocked(ctx, userID, true); httpErr != nil {
		t.Fatal(httpErr)
	}
	if _, httpErr := svc.SetBlocked(ctx, userID, false); httpErr != nil {
		t.Fatal(httpErr)
	}
	if processed, _ := svc.ProcessUserBlockedOutbox(ctx, 10); processed != 0 || notifications.calls != 2 {
		t.Fatalf("event đã huỷ vẫn bị xử lý: processed=%d calls=%d", processed, notifications.calls)
	}
	var pending int64
	db.Model(&model.UserBlockedOutbox{}).Where("user_id = ? AND processed_at IS NULL", userID).Count(&pending)
	if pending != 0 {
		t.Fatalf("còn %d event chờ sau khi mở khoá", pending)
	}
}
