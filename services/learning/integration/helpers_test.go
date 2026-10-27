//go:build integration

// Package integration_test chứa integration test của learning, chạy trên LEARNING_TEST_DSN.
package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"

	identityClient "ego/api/gen/go/identity"
	"ego/services/learning/database"

	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// testIdentityClient cài service.IdentityClient: tên user = id, tìm "teacher" ra teacher-1.
type testIdentityClient struct{}

func (testIdentityClient) GetUsersInfo(_ context.Context, req *identityClient.GetUsersInfoRequest, _ ...grpc.CallOption) (*identityClient.GetUsersInfoResponse, error) {
	users := make([]*identityClient.UserInfo, len(req.Ids))
	for i, id := range req.Ids {
		users[i] = &identityClient.UserInfo{Id: id, Email: id + "@example.com", Name: id}
	}
	return &identityClient.GetUsersInfoResponse{Users: users}, nil
}

func (testIdentityClient) SearchUserIDsByName(_ context.Context, req *identityClient.SearchUserIDsByNameRequest, _ ...grpc.CallOption) (*identityClient.SearchUserIDsByNameResponse, error) {
	if strings.Contains(strings.ToLower(req.Query), "teacher") {
		return &identityClient.SearchUserIDsByNameResponse{Ids: []string{"teacher-1"}}, nil
	}
	return &identityClient.SearchUserIDsByNameResponse{}, nil
}

func learningTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("LEARNING_TEST_DSN")
	if dsn == "" {
		t.Skip("LEARNING_TEST_DSN is not configured")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}
