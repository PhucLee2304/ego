//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	learning "ego/api/gen/go/learning"
	"ego/services/posts/config"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/service"
	"google.golang.org/grpc"
)

// classroomClient cài learning.LearningServiceClient.
type classroomClient struct{}

func (classroomClient) GetClassroomPostAccess(_ context.Context, req *learning.GetClassroomPostAccessRequest, _ ...grpc.CallOption) (*learning.GetClassroomPostAccessResponse, error) {
	return &learning.GetClassroomPostAccessResponse{ClassroomId: req.ClassroomId, ClassroomName: "Class 1", TeacherId: "teacher", Active: true, IsTeacher: req.UserId == "teacher", IsApprovedMember: req.UserId != "teacher", ApprovedStudentIds: []string{"student-a", "student-b"}}, nil
}

func TestInteractionAndPinNotificationsUseDirectOwner(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	svc := service.New(repo, &config.AppConfig{}, classroomClient{}, nil, nil, nil)

	post := &model.Post{AuthorID: "author-b", Scope: model.PostScopeCommunity, Content: "Post", Status: model.PostStatusPublished, AllowComments: true}
	if err := repo.CreatePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := svc.LikePost(ctx, "user-a", post.ID); err != nil {
		t.Fatal(err)
	}
	direct, err := svc.CreatePostComment(ctx, "user-a", post.ID, dto.CreateCommentRequest{Content: "Direct"})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := svc.CreatePostComment(ctx, "user-c", post.ID, dto.CreateCommentRequest{Content: "Reply", ReplyToCommentID: &direct.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.LikePostComment(ctx, "user-c", direct.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.LikePostComment(ctx, "user-c", reply.ID); err != nil {
		t.Fatal(err)
	} // self-like must not notify

	classroomID := uint(9)
	classPost := &model.Post{AuthorID: "teacher", Scope: model.PostScopeClassroom, ClassroomID: &classroomID, Content: "Pinned", Status: model.PostStatusPublished, AllowComments: true}
	if err := repo.CreatePost(ctx, classPost); err != nil {
		t.Fatal(err)
	}
	if err := svc.PinPost(ctx, "teacher", classroomID, classPost.ID); err != nil {
		t.Fatal(err)
	}

	events, claimErr := repo.ClaimOutbox(ctx, time.Now().Add(time.Second), 20)
	if claimErr != nil {
		t.Fatal(claimErr)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 notifications, got %d", len(events))
	}
	recipients := map[string]int{}
	for _, event := range events {
		for _, recipient := range event.Recipients {
			recipients[recipient]++
		}
	}
	if recipients["author-b"] != 2 {
		t.Fatalf("post author notifications = %d", recipients["author-b"])
	}
	if recipients["user-a"] != 2 {
		t.Fatalf("comment author notifications = %d", recipients["user-a"])
	}
	if recipients["student-a"] != 1 || recipients["student-b"] != 1 {
		t.Fatalf("pin recipients = %#v", recipients)
	}
}

func TestUnlikeAndUnpinDoNotCreateNotifications(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	svc := service.New(repo, &config.AppConfig{}, classroomClient{}, nil, nil, nil)
	classroomID := uint(9)
	post := &model.Post{AuthorID: "teacher", Scope: model.PostScopeClassroom, ClassroomID: &classroomID, Content: "Post", Status: model.PostStatusPublished, AllowComments: true, Pinned: true}
	if err := repo.CreatePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LikePost(ctx, post.ID, "student-a"); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnlikePost(ctx, "student-a", post.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnpinPost(ctx, "teacher", classroomID, post.ID); err != nil {
		t.Fatal(err)
	}
	events, err := repo.ClaimOutbox(ctx, time.Now().Add(time.Second), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no notifications, got %d", len(events))
	}
}
