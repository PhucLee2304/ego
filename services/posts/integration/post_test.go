//go:build integration

package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/posts/database"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testRepository(t *testing.T) *repository.Repository {
	t.Helper()
	db, err := gorm.Open(postgres.Open(os.Getenv("POSTS_TEST_DSN")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.NotificationOutbox{}, &model.PostModerationAudit{}, &model.CommentLike{}, &model.PostLike{}, &model.PostComment{}, &model.Post{}); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return repository.NewRepository(db)
}

func TestPostCommentLikesPinAndOutbox(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	classroomID := uint(7)
	post := &model.Post{AuthorID: "teacher", Scope: model.PostScopeClassroom, ClassroomID: &classroomID, Content: "Welcome", Status: model.PostStatusPublished, AllowComments: true}
	if err := repo.Transaction(ctx, func(tx *repository.Repository) error {
		if err := tx.CreatePost(ctx, post); err != nil {
			return err
		}
		return tx.CreateOutbox(ctx, &model.NotificationOutbox{EventKey: "post:created", Recipients: datatypes.JSONSlice[string]{"student-1", "student-2"}, Title: "Class", Body: "New post", Data: datatypes.JSONMap{"id": "1", "type": "CLASSROOM_POST"}, NextAttemptAt: time.Now()})
	}); err != nil {
		t.Fatal(err)
	}
	created, err := repo.LikePost(ctx, post.ID, "student-1")
	if err != nil || !created {
		t.Fatal(err)
	}
	created, err = repo.LikePost(ctx, post.ID, "student-1")
	if err != nil || created {
		t.Fatal(err)
	}
	comment := &model.PostComment{PostID: post.ID, UserID: "student-1", Content: "Thanks"}
	if err := repo.CreateComment(ctx, comment); err != nil {
		t.Fatal(err)
	}
	likes, comments, liked, err := repo.PostStats(ctx, post.ID, "student-1")
	if err != nil || likes != 1 || comments != 1 || !liked {
		t.Fatalf("unexpected stats: likes=%d comments=%d liked=%v err=%v", likes, comments, liked, err)
	}
	events, err := repo.ClaimOutbox(ctx, time.Now().Add(time.Second), 10)
	if err != nil || len(events) != 1 || len(events[0].Recipients) != 2 {
		t.Fatalf("unexpected outbox: %#v err=%v", events, err)
	}
}

func TestSoftDeleteKeepsPostRow(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	post := &model.Post{AuthorID: "user", Scope: model.PostScopeCommunity, Content: "Post", Status: model.PostStatusPublished, AllowComments: true}
	if err := repo.CreatePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeletePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetPost(ctx, post.ID); err == nil {
		t.Fatal("deleted post remained visible")
	}
}

func TestListCommunityPostsByAuthorIncludesHiddenPosts(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	posts := []*model.Post{
		{AuthorID: "owner", Scope: model.PostScopeCommunity, Content: "Published", Status: model.PostStatusPublished, AllowComments: true},
		{AuthorID: "owner", Scope: model.PostScopeCommunity, Content: "Hidden", Status: model.PostStatusHidden, AllowComments: true},
		{AuthorID: "another-user", Scope: model.PostScopeCommunity, Content: "Other", Status: model.PostStatusPublished, AllowComments: true},
	}
	for _, post := range posts {
		if err := repo.CreatePost(ctx, post); err != nil {
			t.Fatal(err)
		}
	}

	result, total, err := repo.ListCommunityPostsByAuthor(ctx, "owner", httpx.PaginationQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(result) != 2 {
		t.Fatalf("expected two owner posts, got total=%d items=%d", total, len(result))
	}
	if result[0].Status != model.PostStatusHidden && result[1].Status != model.PostStatusHidden {
		t.Fatal("hidden owner post was not returned")
	}
}
