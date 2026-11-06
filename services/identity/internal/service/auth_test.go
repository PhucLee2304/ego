package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"ego/platform/firebase"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"

	"gorm.io/gorm"
)

type fakeIdentityProvider struct {
	user *firebase.FirebaseUser
	err  error
}

func (f *fakeIdentityProvider) VerifyIDToken(context.Context, string) (*firebase.FirebaseUser, error) {
	return f.user, f.err
}

type fakeUserRepository struct {
	repository.Repository
	upserted             *model.User
	updates              map[string]any
	user                 *model.User
	err                  error
	transactionCommitted bool
}

func (f *fakeUserRepository) Transaction(ctx context.Context, fn func(repository.Repository) error) error {
	err := fn(f)
	f.transactionCommitted = err == nil
	return err
}

func (f *fakeUserRepository) UpsertUser(_ context.Context, user *model.User) (*model.User, error) {
	f.upserted = user
	return f.user, f.err
}

func (f *fakeUserRepository) GetByID(context.Context, string) (*model.User, error) {
	return f.user, f.err
}

func (f *fakeUserRepository) UpdateMe(_ context.Context, _ string, updates map[string]any) (*model.User, error) {
	f.updates = updates
	return f.user, f.err
}

type fakeTokenManager struct {
	accessToken  string
	refreshToken string
	err          error
}

func (f *fakeTokenManager) Generate(context.Context, string) (string, string, error) {
	return f.accessToken, f.refreshToken, f.err
}

func (f *fakeTokenManager) Validate(context.Context, string) (string, error) {
	return "", nil
}

func (f *fakeTokenManager) ValidateRefresh(context.Context, string) (string, error) {
	return "", nil
}

func TestLoginRejectsMissingIdentityClaims(t *testing.T) {
	tests := []struct {
		name string
		user *firebase.FirebaseUser
	}{
		{name: "missing UID", user: &firebase.FirebaseUser{Email: "user@example.com"}},
		{name: "missing email", user: &firebase.FirebaseUser{UID: "firebase-uid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepository{}
			svc := New(&fakeTokenManager{}, &fakeIdentityProvider{user: tt.user}, repo, nil)

			_, httpErr := svc.Login(context.Background(), dto.LoginBody{IdToken: "valid-token"})
			if httpErr == nil || httpErr.StatusCode != http.StatusUnauthorized {
				t.Fatalf("expected unauthorized, got %#v", httpErr)
			}
			if repo.upserted != nil {
				t.Fatal("repository must not be called for incomplete identity claims")
			}
		})
	}
}

func TestLoginUsesInjectedDependencies(t *testing.T) {
	repo := &fakeUserRepository{user: &model.User{}}
	repo.user.ID = 42
	tokens := &fakeTokenManager{accessToken: "access", refreshToken: "refresh"}
	svc := New(tokens, &fakeIdentityProvider{user: &firebase.FirebaseUser{
		UID:   " firebase-uid ",
		Email: " user@example.com ",
	}}, repo, nil)

	response, httpErr := svc.Login(context.Background(), dto.LoginBody{IdToken: "valid-token"})
	if httpErr != nil {
		t.Fatal(httpErr)
	}
	if response.AccessToken != "access" || response.RefreshToken != "refresh" {
		t.Fatalf("unexpected tokens: %#v", response)
	}
	if repo.upserted.FirebaseUID != "firebase-uid" || repo.upserted.Email != "user@example.com" {
		t.Fatalf("identity claims were not normalized: %#v", repo.upserted)
	}
}

func TestLoginMapsRepositoryFailure(t *testing.T) {
	repoErr := errors.New("database unavailable")
	svc := New(&fakeTokenManager{}, &fakeIdentityProvider{user: &firebase.FirebaseUser{
		UID:   "firebase-uid",
		Email: "user@example.com",
	}}, &fakeUserRepository{err: repoErr}, nil)

	_, httpErr := svc.Login(context.Background(), dto.LoginBody{IdToken: "valid-token"})
	if httpErr == nil || httpErr.StatusCode != http.StatusInternalServerError || !errors.Is(httpErr, repoErr) {
		t.Fatalf("expected wrapped repository error, got %#v", httpErr)
	}
}

func TestLoginRollsBackWhenTokenGenerationFails(t *testing.T) {
	tokenErr := errors.New("failed to sign token")
	repo := &fakeUserRepository{user: &model.User{}}
	repo.user.ID = 42
	svc := New(&fakeTokenManager{err: tokenErr}, &fakeIdentityProvider{user: &firebase.FirebaseUser{
		UID:   "firebase-uid",
		Email: "user@example.com",
	}}, repo, nil)

	_, httpErr := svc.Login(context.Background(), dto.LoginBody{IdToken: "valid-token"})
	if httpErr == nil || !errors.Is(httpErr, tokenErr) {
		t.Fatalf("expected wrapped token error, got %#v", httpErr)
	}
	if repo.transactionCommitted {
		t.Fatal("transaction must be rolled back when token generation fails")
	}
}

func TestLoginRejectsBlockedUserAndRollsBack(t *testing.T) {
	now := time.Now()
	repo := &fakeUserRepository{user: &model.User{BlockedAt: &now}}
	svc := New(&fakeTokenManager{accessToken: "access"}, &fakeIdentityProvider{user: &firebase.FirebaseUser{
		UID:   "firebase-uid",
		Email: "user@example.com",
	}}, repo, nil)

	response, httpErr := svc.Login(context.Background(), dto.LoginBody{IdToken: "valid-token"})
	if httpErr == nil || httpErr.StatusCode != http.StatusForbidden || !errors.Is(httpErr, ErrUserBlocked) {
		t.Fatalf("expected forbidden blocked error, got response=%#v err=%#v", response, httpErr)
	}
	if repo.transactionCommitted {
		t.Fatal("login of a blocked user must not commit anything")
	}
}

func TestRefreshRejectsBlockedUser(t *testing.T) {
	now := time.Now()
	svc := New(&fakeTokenManager{accessToken: "access"}, nil, &fakeUserRepository{user: &model.User{BlockedAt: &now}}, nil)

	_, _, httpErr := svc.Refresh(context.Background(), "refresh")
	if httpErr == nil || httpErr.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %#v", httpErr)
	}
}

func TestRefreshRejectsDeletedUser(t *testing.T) {
	svc := New(&fakeTokenManager{}, nil, &fakeUserRepository{err: gorm.ErrRecordNotFound}, nil)

	_, _, httpErr := svc.Refresh(context.Background(), "refresh")
	if httpErr == nil || httpErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %#v", httpErr)
	}
}

func TestSetBlocked(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name       string
		user       *model.User
		blocked    bool
		wantStatus int
	}{
		{name: "block active client", user: &model.User{Role: model.RoleClient}, blocked: true},
		{name: "unblock blocked client", user: &model.User{Role: model.RoleClient, BlockedAt: &now}, blocked: false},
		{name: "admin cannot be blocked", user: &model.User{Role: model.RoleAdmin}, blocked: true, wantStatus: http.StatusConflict},
		{name: "already blocked", user: &model.User{Role: model.RoleClient, BlockedAt: &now}, blocked: true, wantStatus: http.StatusConflict},
		{name: "not blocked", user: &model.User{Role: model.RoleClient}, blocked: false, wantStatus: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeUserRepository{user: tt.user}
			_, httpErr := New(nil, nil, repo, nil).SetBlocked(context.Background(), "7", tt.blocked)
			if tt.wantStatus != 0 {
				if httpErr == nil || httpErr.StatusCode != tt.wantStatus {
					t.Fatalf("expected status %d, got %#v", tt.wantStatus, httpErr)
				}
				if repo.updates != nil {
					t.Fatal("rejected request must not update the user")
				}
				return
			}
			if httpErr != nil {
				t.Fatal(httpErr)
			}
			blockedAt, ok := repo.updates["blocked_at"].(*time.Time)
			if !ok || (blockedAt != nil) != tt.blocked {
				t.Fatalf("unexpected blocked_at update: %#v", repo.updates)
			}
		})
	}
}
