package dto

import (
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/identity/internal/model"
)

type User struct {
	ID        uint       `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	Avatar    *string    `json:"avatar"`
	Role      string     `json:"role"`
	BlockedAt *time.Time `json:"blockedAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

type UpdateUserBody struct {
	Name   *string `json:"name,omitempty" validate:"omitempty,notblank,max=100"`
	Avatar *string `json:"avatar,omitempty" validate:"omitempty,notblank"`
}

// UsersQuery lọc danh sách người dùng của admin: tìm theo tên/email, theo role, theo trạng thái khoá.
type UsersQuery struct {
	httpx.PaginationQuery
	Query   string `schema:"query" validate:"omitempty,notblank"`
	Role    string `schema:"role" validate:"omitempty,oneof=CLIENT ADMIN"`
	Blocked *bool  `schema:"blocked" validate:"omitempty"`
}

func (q *UsersQuery) Normalize() {
	q.PaginationQuery.Normalize()
	q.Query = strings.TrimSpace(q.Query)
}

// UsersSummary là số liệu trên thẻ tiêu đề màn Users, không phụ thuộc bộ lọc.
type UsersSummary struct {
	Total   int64 `json:"total"`
	Admins  int64 `json:"admins"`
	Blocked int64 `json:"blocked"`
}

func ToUser(user *model.User) *User {
	return &User{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Avatar:    user.Avatar,
		Role:      string(user.Role),
		BlockedAt: user.BlockedAt,
		CreatedAt: user.CreatedAt,
	}
}
