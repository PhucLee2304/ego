//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/platform/httpx"
	"ego/services/posts/internal/model"
)

// GetAdminStats chỉ đếm bài cộng đồng; bài trong lớp chỉ vào ClassroomPosts.
func TestGetAdminStats(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)
	classroomID := uint(3)
	posts := []*model.Post{
		{AuthorID: "a", Scope: model.PostScopeCommunity, Content: "one", Status: model.PostStatusPublished},
		{AuthorID: "a", Scope: model.PostScopeCommunity, Content: "two", Status: model.PostStatusPublished},
		{AuthorID: "b", Scope: model.PostScopeCommunity, Content: "hidden", Status: model.PostStatusHidden},
		{AuthorID: "t", Scope: model.PostScopeClassroom, ClassroomID: &classroomID, Content: "class", Status: model.PostStatusPublished},
	}
	for _, p := range posts {
		if err := repo.CreatePost(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for _, user := range []string{"x", "y"} {
		if _, err := repo.LikePost(ctx, posts[0].ID, user); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.LikePost(ctx, posts[3].ID, "x"); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateComment(ctx, &model.PostComment{PostID: posts[0].ID, UserID: "x", Content: "nice"}); err != nil {
		t.Fatal(err)
	}

	now := time.Now().In(httpx.StatsLocation)
	resp, err := repo.GetAdminStats(ctx, httpx.StatsQuery{From: now.AddDate(0, 0, -6).Format(time.DateOnly), To: now.Format(time.DateOnly)}.Range())
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalPosts != 2 || resp.NewPosts.Current != 3 || resp.NewPosts.Previous != 0 || resp.HiddenPosts != 1 || resp.ClassroomPosts != 1 {
		t.Fatalf("posts = %+v", resp)
	}
	if resp.Comments.Current != 1 || resp.Likes.Current != 2 {
		t.Fatalf("comments=%+v likes=%+v", resp.Comments, resp.Likes)
	}
	if len(resp.Daily) != 1 || resp.Daily[0].Posts != 3 || resp.Daily[0].Comments != 1 {
		t.Fatalf("daily = %+v", resp.Daily)
	}
	if len(resp.TopAuthors) != 2 || resp.TopAuthors[0].UserID != "a" || resp.TopAuthors[0].Posts != 2 || resp.TopAuthors[0].Likes != 2 {
		t.Fatalf("top authors = %+v", resp.TopAuthors)
	}
}
