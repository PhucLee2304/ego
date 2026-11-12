package service

import (
	"context"
	storageClient "ego/api/gen/go/storage"
	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
	"errors"
	"net/http"
	"strings"
	"time"
)

// **Reviewed**
func (s *service) GetMe(ctx context.Context, userID string) (*dto.User, *httpx.HTTPError) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get user")
	}
	return dto.ToUser(user), nil
}

// **Reviewed**
func (s *service) UpdateMe(ctx context.Context, userID string, body dto.UpdateUserBody) (*dto.User, *httpx.HTTPError) {
	updates := map[string]any{}
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Avatar != nil {
		// Avatar là fileName mà storage trả về khi presign, nằm trực tiếp trong avatars/users: không rỗng,
		// không phải "." hay "..", không chứa "/" hoặc "\" để không trỏ ra thư mục khác.
		if fileName := *body.Avatar; fileName == "" || fileName == "." || fileName == ".." || strings.ContainsAny(fileName, `/\`) {
			return nil, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid avatar", nil)
		}
		resp, err := s.storageClient.GetPublicURL(ctx, &storageClient.GetPublicURLRequest{
			Folders:  []string{"avatars", "users"},
			FileName: *body.Avatar,
		})
		if err != nil {
			return nil, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get avatar URL", err)
		}
		updates["avatar"] = resp.Url
	}

	var user *model.User
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		var err error
		user, err = repo.UpdateMe(ctx, userID, updates)
		return err
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update user")
	}
	return dto.ToUser(user), nil
}

// **Reviewed**
func (s *service) GetList(ctx context.Context, query dto.UsersQuery) ([]*dto.User, int64, *httpx.HTTPError) {
	users, total, err := s.repo.GetList(ctx, query)
	if err != nil {
		return nil, 0, httpx.NewError(http.StatusInternalServerError, "[ERROR] Failed to get users", err)
	}

	userDTOs := make([]*dto.User, len(users))
	for i, user := range users {
		userDTOs[i] = dto.ToUser(user)
	}
	return userDTOs, total, nil
}

// SetBlocked khoá (blocked = true) hoặc mở khoá (blocked = false) một tài khoản CLIENT.
//   - không tìm thấy user: 404;
//   - user không phải CLIENT (ví dụ ADMIN): 409, không được khoá hay mở khoá;
//   - khoá tài khoản đang bị khoá: 409 "already blocked";
//   - mở khoá tài khoản không bị khoá: 409 "not blocked";
//   - khoá thành công: ghi event UserBlockedOutbox cùng transaction với blocked_at; worker sẽ báo
//     notifications và learning (xoá token push, đá khỏi phòng live, đóng WebSocket) và tự thử lại khi lỗi;
//   - mở khoá thành công: huỷ event khoá còn chờ xử lý của user.
//
// **Reviewed**
func (s *service) SetBlocked(ctx context.Context, userID string, blocked bool) (*dto.User, *httpx.HTTPError) {
	var user *model.User
	err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		var err error
		user, err = repo.GetByIDForUpdate(ctx, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to get user")
		}
		if user.Role != model.RoleClient {
			return httpx.NewError(http.StatusConflict, "[CONFLICT] Only client accounts can be blocked", nil)
		}
		if blocked && user.BlockedAt != nil {
			return httpx.NewError(http.StatusConflict, "[CONFLICT] Account is already blocked", nil)
		}
		if !blocked && user.BlockedAt == nil {
			return httpx.NewError(http.StatusConflict, "[CONFLICT] Account is not blocked", nil)
		}

		var blockedAt *time.Time
		if blocked {
			now := time.Now()
			blockedAt = &now
		}
		user, err = repo.UpdateMe(ctx, userID, map[string]any{"blocked_at": blockedAt})
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update user")
		}
		if blocked {
			err = repo.CreateUserBlockedOutbox(ctx, &model.UserBlockedOutbox{
				UserID:        userID,
				NextAttemptAt: *blockedAt,
			})
		} else {
			err = repo.CancelUserBlockedOutbox(ctx, userID, time.Now())
		}
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update user")
		}
		return nil
	})
	if err != nil {
		var httpErr *httpx.HTTPError
		if errors.As(err, &httpErr) {
			return nil, httpErr
		}
		return nil, httpx.FromError(err, "[ERROR] Failed to update user")
	}
	return dto.ToUser(user), nil
}
