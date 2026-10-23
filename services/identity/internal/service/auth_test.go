package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"ego/platform/firebase"
	"ego/services/identity/internal/dto"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
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
