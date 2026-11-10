//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"ego/services/posts/internal/model"
)

func TestCleanupBatchQueries(t *testing.T) {
	ctx := context.Background()
	repo := testRepository(t)

	// CancelUploadIntents chỉ đổi intent còn PENDING.
	pending := &model.PostUploadIntent{UserID: "u", Folder: "posts/community", FileName: "a.png", OriginalName: "a.png", ContentType: "image/png", DeclaredSize: 1, MediaType: model.AttachmentTypeImage, Status: model.UploadStatusPending, ExpiresAt: time.Now()}
	completed := &model.PostUploadIntent{UserID: "u", Folder: "posts/community", FileName: "b.png", OriginalName: "b.png", ContentType: "image/png", DeclaredSize: 1, MediaType: model.AttachmentTypeImage, Status: model.UploadStatusCompleted, ExpiresAt: time.Now()}
	for _, intent := range []*model.PostUploadIntent{pending, completed} {
		if err := repo.CreateUploadIntent(ctx, intent); err != nil {
			t.Fatal(err)
		}
	}
	cancelled, err := repo.CancelUploadIntents(ctx, []uint{pending.ID, completed.ID})
	if err != nil || cancelled != 1 {
		t.Fatalf("cancelled=%d err=%v", cancelled, err)
	}
	if got, _ := repo.GetUploadIntent(ctx, completed.ID); got.Status != model.UploadStatusCompleted {
		t.Fatalf("completed intent changed to %s", got.Status)
	}

	// DeleteOrphanAttachmentsPermanently chỉ xóa đính kèm chưa gắn bài, và intent trỏ tới bị SET NULL.
	post := &model.Post{AuthorID: "u", Scope: model.PostScopeCommunity, Content: "x", Status: model.PostStatusPublished}
	if err := repo.CreatePost(ctx, post); err != nil {
		t.Fatal(err)
	}
	orphan := &model.PostAttachment{UserID: "u", URL: "o", ObjectKey: "posts/community/o.png", MediaType: model.AttachmentTypeImage, OriginalName: "o.png", ContentType: "image/png", Size: 1}
	attached := &model.PostAttachment{PostID: &post.ID, UserID: "u", URL: "p", ObjectKey: "posts/community/p.png", MediaType: model.AttachmentTypeImage, OriginalName: "p.png", ContentType: "image/png", Size: 1}
	for _, attachment := range []*model.PostAttachment{orphan, attached} {
		if err := repo.CreateAttachment(ctx, attachment); err != nil {
			t.Fatal(err)
		}
	}
	completed.AttachmentID = &orphan.ID
	if err := repo.SaveUploadIntent(ctx, completed); err != nil {
		t.Fatal(err)
	}
	deleted, err := repo.DeleteOrphanAttachmentsPermanently(ctx, []uint{orphan.ID, attached.ID})
	if err != nil || deleted != 1 {
		t.Fatalf("deleted=%d err=%v", deleted, err)
	}
	if _, err := repo.GetAttachment(ctx, attached.ID); err != nil {
		t.Fatalf("attached attachment was deleted: %v", err)
	}
	if got, _ := repo.GetUploadIntent(ctx, completed.ID); got.AttachmentID != nil {
		t.Fatal("intent still points to deleted attachment")
	}
}
