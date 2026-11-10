package service

import (
	"context"
	"errors"
	"testing"
	"time"

	learningClient "ego/api/gen/go/learning"
	notificationsClient "ego/api/gen/go/notifications"
	"ego/services/identity/internal/model"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

// validatingTokenManager trả userID/lỗi cố định cho Validate.
type validatingTokenManager struct {
	fakeTokenManager
	userID string
	err    error
}

func (f *validatingTokenManager) Validate(context.Context, string) (string, error) {
	return f.userID, f.err
}

func TestValidateToken(t *testing.T) {
	now := time.Now()
	dbErr := errors.New("db down")
	tests := []struct {
		name     string
		tokenErr error
		user     *model.User
		repoErr  error
		want     error
	}{
		{name: "hợp lệ", user: &model.User{}},
		{name: "token sai", tokenErr: errors.New("expired"), want: ErrInvalidToken},
		{name: "user không tồn tại", repoErr: gorm.ErrRecordNotFound, want: ErrUserNotFound},
		{name: "user bị khoá", user: &model.User{BlockedAt: &now}, want: ErrUserBlocked},
		{name: "lỗi DB", repoErr: dbErr, want: dbErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := New(&validatingTokenManager{userID: "7", err: tt.tokenErr}, nil, &fakeUserRepository{user: tt.user, err: tt.repoErr}, nil, nil, nil)
			userID, err := svc.ValidateToken(context.Background(), "token")
			if !errors.Is(err, tt.want) || (tt.want == nil && userID != "7") {
				t.Fatalf("userID=%q err=%v, want err %v", userID, err, tt.want)
			}
		})
	}
}

type fakeNotificationsClient struct {
	deletedFor []string
	err        error
}

func (f *fakeNotificationsClient) DeleteDeviceTokensByUserID(_ context.Context, in *notificationsClient.DeleteDeviceTokensByUserIDRequest, _ ...grpc.CallOption) (*notificationsClient.DeleteDeviceTokensByUserIDResponse, error) {
	f.deletedFor = append(f.deletedFor, in.UserId)
	return &notificationsClient.DeleteDeviceTokensByUserIDResponse{}, f.err
}

type fakeLearningClient struct {
	removedFor []string
	err        error
}

func (f *fakeLearningClient) RemoveUserFromLiveRooms(_ context.Context, in *learningClient.RemoveUserFromLiveRoomsRequest, _ ...grpc.CallOption) (*learningClient.RemoveUserFromLiveRoomsResponse, error) {
	f.removedFor = append(f.removedFor, in.UserId)
	return &learningClient.RemoveUserFromLiveRoomsResponse{}, f.err
}

// TestSetBlockedWritesOutbox: khoá thì ghi event outbox trong transaction (không gọi rpc trực tiếp),
// mở khoá thì huỷ event còn chờ.
func TestSetBlockedWritesOutbox(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name          string
		blocked       bool
		user          *model.User
		wantOutbox    bool
		wantCancelled bool
	}{
		{name: "khoá thì ghi outbox", blocked: true, user: &model.User{Role: model.RoleClient}, wantOutbox: true},
		{name: "mở khoá thì huỷ outbox", blocked: false, user: &model.User{Role: model.RoleClient, BlockedAt: &now}, wantCancelled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepository{user: tt.user}
			notifications := &fakeNotificationsClient{}
			_, httpErr := New(nil, nil, repo, nil, notifications, &fakeLearningClient{}).SetBlocked(context.Background(), "7", tt.blocked)
			if httpErr != nil {
				t.Fatal(httpErr)
			}
			gotOutbox := repo.outboxCreated != nil && repo.outboxCreated.UserID == "7"
			if gotOutbox != tt.wantOutbox || (repo.outboxCancelledFor == "7") != tt.wantCancelled {
				t.Fatalf("outbox=%+v cancelled=%q", repo.outboxCreated, repo.outboxCancelledFor)
			}
			if !repo.transactionCommitted || len(notifications.deletedFor) != 0 {
				t.Fatalf("committed=%v rpc gọi trực tiếp=%v", repo.transactionCommitted, notifications.deletedFor)
			}
		})
	}
}

// TestProcessUserBlockedOutbox: chỉ đánh dấu xong khi cả hai rpc thành công; một rpc lỗi thì hẹn thử lại.
func TestProcessUserBlockedOutbox(t *testing.T) {
	tests := []struct {
		name          string
		notifyErr     error
		learningErr   error
		wantProcessed bool
	}{
		{name: "cả hai thành công", wantProcessed: true},
		{name: "notifications lỗi", notifyErr: errors.New("unavailable")},
		{name: "learning lỗi", learningErr: errors.New("unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepository{claimed: []*model.UserBlockedOutbox{{Model: gorm.Model{ID: 3}, UserID: "7", Attempts: 1}}}
			notifications := &fakeNotificationsClient{err: tt.notifyErr}
			learning := &fakeLearningClient{err: tt.learningErr}
			processed, err := New(nil, nil, repo, nil, notifications, learning).ProcessUserBlockedOutbox(context.Background(), 10)
			if err != nil {
				t.Fatal(err)
			}
			// Gọi cả hai rpc kể cả khi một bên lỗi, để bên còn lại không phải chờ lần thử sau.
			if len(notifications.deletedFor) != 1 || len(learning.removedFor) != 1 {
				t.Fatalf("notifications=%v learning=%v, want mỗi bên 1 lần", notifications.deletedFor, learning.removedFor)
			}
			if tt.wantProcessed {
				if processed != 1 || repo.processedID != 3 || repo.failedID != 0 {
					t.Fatalf("processed=%d processedID=%d failedID=%d", processed, repo.processedID, repo.failedID)
				}
				return
			}
			if processed != 0 || repo.processedID != 0 || repo.failedID != 3 || repo.failedAttempts != 2 {
				t.Fatalf("processed=%d processedID=%d failedID=%d attempts=%d", processed, repo.processedID, repo.failedID, repo.failedAttempts)
			}
		})
	}
}
