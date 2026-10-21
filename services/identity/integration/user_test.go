//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	identityClient "ego/api/gen/go/identity"
	"ego/platform/firebase"
	"ego/platform/jwt"
	platformRpc "ego/platform/rpc"
	"ego/services/identity/database"
	"ego/services/identity/internal/model"
	"ego/services/identity/internal/repository"
	"ego/services/identity/internal/service"
	identityRpc "ego/services/identity/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testService(t *testing.T) (service.Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(os.Getenv("IDENTITY_TEST_DSN")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.User{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	// GetRole, GetUsersInfo and SearchUserIDsByName only use the repository.
	return service.New(nil, firebase.Client{}, repository.NewRepository(db), nil), db
}

func createUser(t *testing.T, db *gorm.DB, email, name string, role model.RoleName) string {
	t.Helper()
	user := &model.User{FirebaseUID: "firebase-" + email, Email: email, Name: name, Role: role}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(int(user.ID))
}

func TestGetRole(t *testing.T) {
	svc, db := testService(t)
	adminID := createUser(t, db, "admin@example.com", "Admin", model.RoleAdmin)

	role, err := svc.GetRole(context.Background(), adminID)
	if err != nil || role != string(model.RoleAdmin) {
		t.Fatalf("expected ADMIN, got role=%q err=%v", role, err)
	}

	_, err = svc.GetRole(context.Background(), "999999")
	if !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("GetRole must not leak gorm.ErrRecordNotFound")
	}

	_, err = svc.GetRole(context.Background(), "abc")
	if err == nil || errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("expected a database error for a non-numeric id, got %v", err)
	}
}

// The identity gRPC server wraps every handler with ErrorInterceptor; this runs
// the same chain against the real database.
func TestGetUserRoleRPC(t *testing.T) {
	svc, db := testService(t)
	userID := createUser(t, db, "client@example.com", "Client", model.RoleClient)
	srv := identityRpc.New(svc)
	interceptor := platformRpc.ErrorInterceptor()
	info := &grpc.UnaryServerInfo{FullMethod: "/identity.IdentityService/GetUserRole"}
	call := func(id string) (any, error) {
		return interceptor(context.Background(), &identityClient.GetUserRoleRequest{Id: id}, info, func(ctx context.Context, req any) (any, error) {
			return srv.GetUserRole(ctx, req.(*identityClient.GetUserRoleRequest))
		})
	}

	resp, err := call(userID)
	if err != nil || resp.(*identityClient.GetUserRoleResponse).GetRole() != string(model.RoleClient) {
		t.Fatalf("expected CLIENT, got resp=%#v err=%v", resp, err)
	}

	_, err = call("999999")
	if status.Code(err) != codes.NotFound || status.Convert(err).Message() != "[ERROR] User not found" {
		t.Fatalf("expected NotFound \"[ERROR] User not found\", got %v", err)
	}

	_, err = call("abc")
	if status.Code(err) != codes.Internal || status.Convert(err).Message() != "internal error" {
		t.Fatalf("expected hidden Internal error, got %v", err)
	}
}

func TestRoleMiddlewareWithGetRole(t *testing.T) {
	svc, db := testService(t)
	adminID := createUser(t, db, "admin@example.com", "Admin", model.RoleAdmin)
	clientID := createUser(t, db, "client@example.com", "Client", model.RoleClient)
	handler := jwt.NewRoleMiddleware(svc.GetRole).RequireRole("admin")(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	authenticated := jwt.NewAuthMiddlewareWithValidator(func(_ context.Context, token string) (string, error) {
		return token, nil
	})

	tests := []struct {
		name   string
		userID string
		code   int
	}{
		{name: "admin", userID: adminID, code: http.StatusNoContent},
		{name: "not admin", userID: clientID, code: http.StatusForbidden},
		{name: "unknown user", userID: "999999", code: http.StatusUnauthorized},
		{name: "database error", userID: "abc", code: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/users", nil)
			request.Header.Set("Authorization", "Bearer "+tt.userID)
			recorder := httptest.NewRecorder()
			authenticated.Handle(handler)(recorder, request)
			if recorder.Code != tt.code {
				t.Fatalf("expected %d, got %d: %s", tt.code, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestUserListQueries(t *testing.T) {
	svc, db := testService(t)
	annaID := createUser(t, db, "anna@example.com", "Anna", model.RoleClient)
	createUser(t, db, "bob@example.com", "Bob", model.RoleClient)

	users, err := svc.GetUsersInfo(context.Background(), []string{annaID, "999999"})
	if err != nil || len(users) != 1 || users[0].Name != "Anna" {
		t.Fatalf("expected only Anna, got users=%#v err=%v", users, err)
	}

	ids, err := svc.SearchUserIDsByName(context.Background(), "  ann ")
	if err != nil || len(ids) != 1 || ids[0] != annaID {
		t.Fatalf("expected [%s], got ids=%#v err=%v", annaID, ids, err)
	}
}

func TestUpsertUserUsesFirebaseUIDAndUpdatesEmail(t *testing.T) {
	_, db := testService(t)
	repo := repository.NewRepository(db)
	ctx := context.Background()

	first, err := repo.UpsertUser(ctx, &model.User{
		FirebaseUID: "firebase-uid-1",
		Email:       "old@example.com",
		Name:        "Original Name",
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := repo.UpsertUser(ctx, &model.User{
		FirebaseUID: "firebase-uid-1",
		Email:       "new@example.com",
		Name:        "Provider Name",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected the same user ID, got %d and %d", first.ID, second.ID)
	}
	if second.Email != "new@example.com" {
		t.Fatalf("expected updated email, got %q", second.Email)
	}
	if second.Name != "Original Name" {
		t.Fatalf("expected user-managed name to be preserved, got %q", second.Name)
	}
}

func TestGetListOrdersUsersByID(t *testing.T) {
	_, db := testService(t)
	createUser(t, db, "third@example.com", "Third", model.RoleClient)
	createUser(t, db, "first@example.com", "First", model.RoleClient)
	createUser(t, db, "second@example.com", "Second", model.RoleClient)

	users, _, err := repository.NewRepository(db).GetList(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(users); i++ {
		if users[i-1].ID >= users[i].ID {
			t.Fatalf("expected ascending IDs, got %d before %d", users[i-1].ID, users[i].ID)
		}
	}
}
