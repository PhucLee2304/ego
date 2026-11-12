package rpc

import (
	"context"
	"errors"

	notificationsClient "ego/api/gen/go/notifications"
	platformRpc "ego/platform/rpc"
	"ego/services/notifications/internal/dto"
	"ego/services/notifications/internal/service"

	"google.golang.org/grpc/codes"
)

type server struct {
	notificationsClient.UnimplementedNotificationServiceServer
	service        service.Service
	disconnectUser func(userID string) int // disconnectUser đóng mọi WebSocket đang mở của user; nil thì bỏ qua (test).
}

// New nhận thêm disconnectUser để đóng WebSocket của user bị khoá (xem rpc gọi từ identity SetBlocked).
func New(svc service.Service, disconnectUser func(userID string) int) notificationsClient.NotificationServiceServer {
	return &server{service: svc, disconnectUser: disconnectUser}
}

// CreateNotifications tạo thông báo cho nhiều người nhận từ một event. Bên dùng: worker outbox của
// posts (like, comment, bài viết trong lớp) và learning (lớp học, bài tập, phòng thi, kết quả thi).
// Request sai trả InvalidArgument; lỗi khác trả nguyên để ErrorInterceptor log và che thành Internal,
// bên gọi giữ event trong outbox và thử lại.
func (s *server) CreateNotifications(ctx context.Context, req *notificationsClient.CreateNotificationsRequest) (*notificationsClient.CreateNotificationsResponse, error) {
	created, err := s.service.CreateNotifications(ctx, dto.CreateNotificationsRequest{
		Recipients:     req.Recipients,
		Title:          req.Title,
		Body:           req.Body,
		SourceService:  req.SourceService,
		IdempotencyKey: req.IdempotencyKey,
		Data:           req.Data,
	})
	if errors.Is(err, service.ErrInvalidNotification) {
		return nil, platformRpc.PublicError(codes.InvalidArgument, err.Error())
	}
	if err != nil {
		return nil, err
	}
	return &notificationsClient.CreateNotificationsResponse{CreatedCount: uint32(created)}, nil
}

// DeleteDeviceTokensByUserID xoá mọi token push của một user. Bên dùng: identity (SetBlocked) gọi sau
// khi khoá tài khoản để máy của user không nhận push nữa, và đóng WebSocket thông báo của user. userID rỗng trả InvalidArgument.
func (s *server) DeleteDeviceTokensByUserID(ctx context.Context, req *notificationsClient.DeleteDeviceTokensByUserIDRequest) (*notificationsClient.DeleteDeviceTokensByUserIDResponse, error) {
	deleted, err := s.service.DeleteDeviceTokensByUserID(ctx, req.UserId)
	if errors.Is(err, service.ErrInvalidNotification) {
		return nil, platformRpc.PublicError(codes.InvalidArgument, err.Error())
	}
	// Đóng WebSocket thông báo kể cả khi xoá token lỗi: kết nối cũ không bị kiểm tra token lại.
	if s.disconnectUser != nil {
		s.disconnectUser(req.UserId)
	}
	if err != nil {
		return nil, err
	}
	return &notificationsClient.DeleteDeviceTokensByUserIDResponse{DeletedCount: uint32(deleted)}, nil
}
