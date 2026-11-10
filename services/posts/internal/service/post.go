package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func (s *service) CreateCommunityPost(ctx context.Context, userID string, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	if req.Content == "" && len(req.AttachmentIDs) == 0 {
		return nil, ErrPostEmpty
	}
	post := &model.Post{
		AuthorID:      userID,
		Scope:         model.PostScopeCommunity,
		Content:       req.Content,
		Status:        model.PostStatusPublished,
		AllowComments: req.CommentsAllowed(),
	}
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		if err := repo.CreatePost(ctx, post); err != nil {
			return err
		}
		_, err := s.attachToPost(ctx, repo, userID, post.ID, req.AttachmentIDs)
		return err
	}); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create community post")
	}
	out, err := s.hydratePosts(ctx, []*model.Post{post}, userID, false)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create community post")
	}
	return out[0], nil
}

func (s *service) CreateClassroomPost(ctx context.Context, userID string, classroomID uint, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	if req.Content == "" && len(req.AttachmentIDs) == 0 {
		return nil, ErrPostEmpty
	}
	access, err := s.ensureBelongsToClassroom(ctx, classroomID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create classroom post")
	}
	if !access.IsTeacher {
		return nil, ErrForbidden
	}
	if !access.Active {
		return nil, ErrClassroomReadOnly
	}
	post := &model.Post{
		AuthorID:      userID,
		Scope:         model.PostScopeClassroom,
		ClassroomID:   &classroomID,
		Content:       req.Content,
		Status:        model.PostStatusPublished,
		AllowComments: req.CommentsAllowed(),
	}
	err = s.repo.Transaction(ctx, func(repo repository.Repository) error {
		if err := repo.CreatePost(ctx, post); err != nil {
			return err
		}
		if _, err := s.attachToPost(ctx, repo, userID, post.ID, req.AttachmentIDs); err != nil {
			return err
		}
		if len(access.ApprovedStudentIds) == 0 {
			return nil
		}
		return repo.CreateOutbox(ctx, &model.NotificationOutbox{
			EventKey:   fmt.Sprintf("classroom-post:%d:created", post.ID),
			Recipients: datatypes.JSONSlice[string](access.ApprovedStudentIds),
			Title:      access.ClassroomName,
			Body:       "A new post was published in your classroom.",
			Data: datatypes.JSONMap{
				"id":          fmt.Sprint(post.ID),
				"type":        "CLASSROOM_POST",
				"classroomId": fmt.Sprint(classroomID),
			},
			NextAttemptAt: time.Now(),
		})
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create classroom post")
	}
	out, err := s.hydratePosts(ctx, []*model.Post{post}, userID, true)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create classroom post")
	}
	return out[0], nil
}

func (s *service) ListCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, model.PostStatusPublished, nil, q.Limit(), q.Offset())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list community posts")
	}
	out, err := s.hydratePosts(ctx, items, userID, false)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list community posts")
	}
	return dto.ToPostsResponse(out, total, int(q.Page), int(q.PageSize)), nil
}

func (s *service) ListMyCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	items, total, err := s.repo.ListCommunityPostsByAuthor(ctx, userID, q.Limit(), q.Offset())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list user posts")
	}
	out, err := s.hydratePosts(ctx, items, userID, false)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list user posts")
	}
	return dto.ToPostsResponse(out, total, int(q.Page), int(q.PageSize)), nil
}

func (s *service) ListClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, false)
}

func (s *service) ListPinnedClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, true)
}

func (s *service) listClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery, pinnedOnly bool) (*dto.PostsResponse, *httpx.HTTPError) {
	access, err := s.ensureBelongsToClassroom(ctx, classroomID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list classroom posts")
	}
	q.Normalize()
	var pinned *bool
	if pinnedOnly {
		pinned = &pinnedOnly
	}
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeClassroom, &classroomID, model.PostStatusPublished, pinned, q.Limit(), q.Offset())
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list classroom posts")
	}
	out, err := s.hydratePosts(ctx, items, userID, access.IsTeacher)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list classroom posts")
	}
	return dto.ToPostsResponse(out, total, int(q.Page), int(q.PageSize)), nil
}

func (s *service) GetPost(ctx context.Context, userID string, id uint) (*dto.PostResponse, *httpx.HTTPError) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post")
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to get post")
		}
		teacher = access.IsTeacher
	}
	if post.Status != model.PostStatusPublished && post.AuthorID != userID {
		return nil, ErrNotFound
	}
	out, err := s.hydratePosts(ctx, []*model.Post{post}, userID, teacher)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post")
	}
	return out[0], nil
}

func (s *service) UpdatePost(ctx context.Context, userID string, id uint, req dto.UpdatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post")
	}
	if post.AuthorID != userID {
		return nil, ErrForbidden
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to update post")
		}
		if !access.Active {
			return nil, ErrClassroomReadOnly
		}
		teacher = access.IsTeacher
	}
	if err := s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// Khóa và đọc lại bài để không ghi đè ghim/ẩn mà giáo viên hoặc admin vừa đổi.
		var err error
		post, err = repo.GetPostForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if req.Content != nil {
			post.Content = *req.Content
		}
		if req.AllowComments != nil {
			post.AllowComments = *req.AllowComments
		}
		// Đưa đính kèm về đúng danh sách gửi lên: gỡ cái không còn, gắn cái mới.
		var attachments []*model.PostAttachment
		if req.AttachmentIDs != nil {
			current, err := repo.ListAttachments(ctx, []uint{post.ID})
			if err != nil {
				return err
			}
			wanted := make(map[uint]struct{}, len(*req.AttachmentIDs))
			for _, id := range *req.AttachmentIDs {
				wanted[id] = struct{}{}
			}
			existing := make(map[uint]struct{}, len(current))
			var removed []uint
			for _, attachment := range current {
				existing[attachment.ID] = struct{}{}
				if _, ok := wanted[attachment.ID]; !ok {
					removed = append(removed, attachment.ID)
				}
			}
			var added []uint
			for id := range wanted {
				if _, ok := existing[id]; !ok {
					added = append(added, id)
				}
			}
			if err := repo.DetachFromPost(ctx, post.ID, removed); err != nil {
				return err
			}
			if attachments, err = s.attachToPost(ctx, repo, userID, post.ID, added); err != nil {
				return err
			}
		}
		if strings.TrimSpace(post.Content) == "" {
			// Chưa đồng bộ đính kèm ở trên thì đọc danh sách hiện có.
			if req.AttachmentIDs == nil {
				if attachments, err = repo.ListAttachments(ctx, []uint{post.ID}); err != nil {
					return err
				}
			}
			if len(attachments) == 0 {
				return ErrPostEmpty
			}
		}
		return repo.SavePost(ctx, post)
	}); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post")
	}
	out, err := s.hydratePosts(ctx, []*model.Post{post}, userID, teacher)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post")
	}
	return out[0], nil
}

// attachToPost gắn đính kèm vào bài rồi kiểm tra giới hạn của cả bài (số lượng, tổng dung lượng, tối đa một video);
// trả về toàn bộ đính kèm của bài sau khi gắn.
func (s *service) attachToPost(ctx context.Context, repo repository.Repository, userID string, postID uint, ids []uint) ([]*model.PostAttachment, error) {
	if err := repo.AttachToPost(ctx, ids, userID, postID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAttachment
		}
		return nil, err
	}
	attachments, err := repo.ListAttachments(ctx, []uint{postID})
	if err != nil {
		return nil, err
	}
	var total int64
	videoCount := 0
	for _, attachment := range attachments {
		total += attachment.Size
		if attachment.MediaType == model.AttachmentTypeVideo {
			videoCount++
		}
	}
	if len(attachments) > s.cfg.MaxAttachments || total > s.cfg.PostMaxBytes || videoCount > 1 {
		return nil, ErrInvalidAttachment
	}
	return attachments, nil
}

func (s *service) DeletePost(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to delete post")
	}
	if post.AuthorID != userID {
		return ErrForbidden
	}
	if post.Scope == model.PostScopeClassroom {
		access, err := s.ensureBelongsToClassroom(ctx, *post.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to delete post")
		}
		if !access.Active {
			return ErrClassroomReadOnly
		}
	}
	return httpx.FromError(s.repo.DeletePost(ctx, post), "[ERROR] Failed to delete post")
}

func (s *service) PinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError {
	return s.setPostPinned(ctx, userID, classroomID, postID, true)
}

func (s *service) UnpinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError {
	return s.setPostPinned(ctx, userID, classroomID, postID, false)
}

func (s *service) setPostPinned(ctx context.Context, userID string, classroomID, postID uint, pinned bool) *httpx.HTTPError {
	access, err := s.ensureBelongsToClassroom(ctx, classroomID, userID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update post pin")
	}
	if !access.IsTeacher {
		return ErrForbidden
	}
	if !access.Active {
		return ErrClassroomReadOnly
	}
	return httpx.FromError(s.repo.Transaction(ctx, func(repo repository.Repository) error {
		post, err := repo.GetPostForUpdate(ctx, postID)
		if err != nil {
			return err
		}
		if post.ClassroomID == nil || *post.ClassroomID != classroomID {
			return ErrNotFound
		}
		if post.Pinned == pinned {
			return nil
		}
		post.Pinned = pinned
		if err := repo.SavePost(ctx, post); err != nil {
			return err
		}
		if !pinned || len(access.ApprovedStudentIds) == 0 {
			return nil
		}
		return repo.CreateOutbox(ctx, &model.NotificationOutbox{
			EventKey:      fmt.Sprintf("classroom-post:%d:pinned:%d", post.ID, post.UpdatedAt.UnixNano()),
			Recipients:    datatypes.JSONSlice[string](access.ApprovedStudentIds),
			Title:         access.ClassroomName,
			Body:          "A post was pinned in your classroom.",
			Data:          datatypes.JSONMap{"id": fmt.Sprint(post.ID), "type": "CLASSROOM_POST_PINNED", "classroomId": fmt.Sprint(classroomID)},
			NextAttemptAt: time.Now(),
		})
	}), "[ERROR] Failed to update post pin")
}
