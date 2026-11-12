package rpc

import (
	"context"
	"errors"

	learningClient "ego/api/gen/go/learning"
	platformRpc "ego/platform/rpc"
	"ego/services/learning/internal/service"

	"google.golang.org/grpc/codes"
)

// RemoveUserFromLiveRooms đưa user ra khỏi các phòng LIVE. Bên dùng: identity (SetBlocked) gọi sau
// khi khoá tài khoản; user là giáo viên thì phòng bị kết thúc, và mọi WebSocket của user bị đóng. Thiếu user id trả InvalidArgument.
func (s *server) RemoveUserFromLiveRooms(ctx context.Context, req *learningClient.RemoveUserFromLiveRoomsRequest) (*learningClient.RemoveUserFromLiveRoomsResponse, error) {
	removed, ended, err := s.service.RemoveUserFromLiveRooms(ctx, req.UserId)
	if errors.Is(err, service.ErrInvalidUserID) {
		return nil, platformRpc.PublicError(codes.InvalidArgument, "[ERROR] User id is required")
	}
	// Đóng WebSocket bài thi và lớp học kể cả khi gỡ phòng lỗi: kết nối cũ không bị kiểm tra token lại.
	if s.disconnectUser != nil {
		s.disconnectUser(req.UserId)
	}
	if err != nil {
		return nil, err
	}
	return &learningClient.RemoveUserFromLiveRoomsResponse{RemovedCount: uint32(removed), EndedCount: uint32(ended)}, nil
}
