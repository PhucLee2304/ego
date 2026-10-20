package rpc

import (
	"context"
	"errors"
	"testing"

	identityClient "ego/api/gen/go/identity"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type fakeService struct {
	service.Service
	validateUserID string
	validateErr    error
	role           string
	roleErr        error
	users          []*model.User
	usersErr       error
	ids            []string
	idsErr         error
}

func (f *fakeService) ValidateToken(context.Context, string) (string, error) {
	return f.validateUserID, f.validateErr
}

func (f *fakeService) GetRole(context.Context, string) (string, error) {
	return f.role, f.roleErr
}

func (f *fakeService) GetUsersInfo(context.Context, []string) ([]*model.User, error) {
	return f.users, f.usersErr
}

func (f *fakeService) SearchUserIDsByName(context.Context, string) ([]string, error) {
	return f.ids, f.idsErr
}

func TestValidateTokenReturnsInvalidResponseWithoutRPCError(t *testing.T) {
	srv := New(&fakeService{validateErr: errors.New("invalid token")})

	response, err := srv.ValidateToken(context.Background(), &identityClient.ValidateTokenRequest{Token: "bad-token"})
	if err != nil {
		t.Fatalf("expected no RPC error, got %v", err)
	}
	if response.GetIsValid() || response.GetUserId() != "" {
		t.Fatalf("expected invalid response, got %#v", response)
	}
}

func TestValidateTokenReturnsUserID(t *testing.T) {
	srv := New(&fakeService{validateUserID: "42"})

	response, err := srv.ValidateToken(context.Background(), &identityClient.ValidateTokenRequest{Token: "good-token"})
	if err != nil {
		t.Fatal(err)
	}
	if !response.GetIsValid() || response.GetUserId() != "42" {
		t.Fatalf("expected valid response for user 42, got %#v", response)
	}
}

func TestGetUserRoleMapsNotFound(t *testing.T) {
	srv := New(&fakeService{roleErr: service.ErrUserNotFound})

	_, err := srv.GetUserRole(context.Background(), &identityClient.GetUserRoleRequest{Id: "1"})
	if got := status.Code(err); got != codes.NotFound {
		t.Fatalf("expected NotFound, got %s", got)
	}
	if got := status.Convert(err).Message(); got != "[ERROR] User not found" {
		t.Fatalf("unexpected message %q", got)
	}
}

// Errors that are not domain errors are returned unchanged; the server's
// platformRpc.ErrorInterceptor logs them and hides them behind codes.Internal.
func TestRPCsReturnOtherErrorsUnchanged(t *testing.T) {
	dbErr := errors.New("db down")
	srv := New(&fakeService{roleErr: dbErr, usersErr: dbErr, idsErr: dbErr})

	_, err := srv.GetUserRole(context.Background(), &identityClient.GetUserRoleRequest{Id: "1"})
	if !errors.Is(err, dbErr) {
		t.Fatalf("GetUserRole: expected service error, got %v", err)
	}
	_, err = srv.GetUsersInfo(context.Background(), &identityClient.GetUsersInfoRequest{Ids: []string{"1"}})
	if !errors.Is(err, dbErr) {
		t.Fatalf("GetUsersInfo: expected service error, got %v", err)
	}
	_, err = srv.SearchUserIDsByName(context.Background(), &identityClient.SearchUserIDsByNameRequest{Query: "an"})
	if !errors.Is(err, dbErr) {
		t.Fatalf("SearchUserIDsByName: expected service error, got %v", err)
	}
}

func TestGetUsersInfoMapsUsersAndNilAvatar(t *testing.T) {
	avatar := "https://example.com/avatar.png"
	srv := New(&fakeService{users: []*model.User{
		{Model: gorm.Model{ID: 7}, Email: "one@example.com", Name: "One", Avatar: nil},
		{Model: gorm.Model{ID: 8}, Email: "two@example.com", Name: "Two", Avatar: &avatar},
	}})

	response, err := srv.GetUsersInfo(context.Background(), &identityClient.GetUsersInfoRequest{Ids: []string{"7", "8"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(response.Users))
	}
	if response.Users[0].GetId() != "7" || response.Users[0].GetAvatar() != "" {
		t.Fatalf("unexpected nil-avatar mapping: %#v", response.Users[0])
	}
	if response.Users[1].GetId() != "8" || response.Users[1].GetAvatar() != avatar {
		t.Fatalf("unexpected avatar mapping: %#v", response.Users[1])
	}
}

func TestSearchUserIDsByNameMapsServiceResponse(t *testing.T) {
	srv := New(&fakeService{ids: []string{"3", "5"}})

	response, err := srv.SearchUserIDsByName(context.Background(), &identityClient.SearchUserIDsByNameRequest{Query: "an"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Ids) != 2 || response.Ids[0] != "3" || response.Ids[1] != "5" {
		t.Fatalf("unexpected IDs: %#v", response.Ids)
	}
}
