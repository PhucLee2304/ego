package service

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	classrooms "ego/api/gen/go/classrooms"
	identityClient "ego/api/gen/go/identity"
	notifications "ego/api/gen/go/notifications"
	storage "ego/api/gen/go/storage"
	"ego/platform/httpx"
	postsConfig "ego/services/posts/config"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrNotFound          = httpx.NewError(http.StatusNotFound, "[ERROR] Resource not found", nil)
	ErrForbidden         = httpx.NewError(http.StatusForbidden, "[FORBIDDEN] You do not have permission", nil)
	ErrClassroomReadOnly = httpx.NewError(http.StatusBadRequest, "[ERROR] Archived classroom posts are read-only", nil)
	ErrCommentsDisabled  = httpx.NewError(http.StatusBadRequest, "[ERROR] Comments are disabled", nil)
	ErrInvalidReply      = httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid reply target", nil)
	ErrInvalidAttachment = httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attachment", nil)
	ErrUploadExpired     = httpx.NewError(http.StatusBadRequest, "[ERROR] Upload has expired", nil)
	ErrPostEmpty         = httpx.NewError(http.StatusBadRequest, "[ERROR] Post content or attachment is required", nil)
)

type Service struct {
	repo          *repository.Repository
	classrooms    classrooms.ClassroomServiceClient
	identity      identityClient.IdentityServiceClient
	notifications notifications.NotificationServiceClient
	storage       storage.StorageServiceClient
	cfg           *postsConfig.AppConfig
}

func New(
	repo *repository.Repository,
	appConfig *postsConfig.AppConfig,
	classroomsClient classrooms.ClassroomServiceClient,
	identityClient identityClient.IdentityServiceClient,
	notificationsClient notifications.NotificationServiceClient,
	storageClient storage.StorageServiceClient,
) *Service {
	return &Service{
		repo:          repo,
		classrooms:    classroomsClient,
		identity:      identityClient,
		notifications: notificationsClient,
		storage:       storageClient,
		cfg:           appConfig,
	}
}

var imageContentTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
var videoContentTypes = map[string]bool{"video/mp4": true, "video/quicktime": true}
var fileContentTypes = map[string]bool{
	"application/pdf":    true,
	"text/plain":         true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
}

func (s *Service) UploadLimits() *dto.UploadLimitsResponse {
	return dto.ToUploadLimitsResponse(s.cfg, imageContentTypes, fileContentTypes, videoContentTypes)
}

func (s *Service) CreateUploadIntent(ctx context.Context, userID string, req dto.CreateUploadIntentRequest) (*dto.UploadIntentResponse, *httpx.HTTPError) {
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(req.ContentType, ";")[0]))
	mediaType := model.AttachmentTypeFile
	maxSize := s.cfg.FileMaxBytes
	if imageContentTypes[contentType] {
		mediaType = model.AttachmentTypeImage
		maxSize = s.cfg.ImageMaxBytes
	} else if videoContentTypes[contentType] {
		mediaType, maxSize = model.AttachmentTypeVideo, s.cfg.VideoMaxBytes
	} else if !fileContentTypes[contentType] {
		return nil, ErrInvalidAttachment
	}
	if req.Size > maxSize {
		return nil, ErrInvalidAttachment
	}
	ext := strings.ToLower(filepath.Ext(req.OriginalName))
	fileName := uuid.NewString() + ext
	folder := "posts/uploads/" + userID
	response, err := s.storage.GeneratePresignedUploadURL(ctx, &storage.GeneratePresignedUploadURLRequest{Folders: strings.Split(folder, "/"), FileName: fileName, ContentType: contentType})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create upload intent")
	}
	intent := &model.PostUploadIntent{UserID: userID, Folder: folder, FileName: fileName, OriginalName: req.OriginalName, ContentType: contentType, DeclaredSize: req.Size, MediaType: mediaType, Status: model.UploadStatusPending, ExpiresAt: time.Now().Add(s.cfg.UploadIntentTTL)}
	if err := s.repo.CreateUploadIntent(ctx, intent); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create upload intent")
	}
	return &dto.UploadIntentResponse{ID: intent.ID, UploadURL: response.Url, ContentType: contentType, ExpiresAt: intent.ExpiresAt}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, userID string, id uint) (*dto.AttachmentResponse, *httpx.HTTPError) {
	var result *model.PostAttachment
	err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		intent, err := repo.GetUploadIntentForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if intent.UserID != userID {
			return ErrForbidden
		}
		if intent.Status == model.UploadStatusCompleted && intent.AttachmentID != nil {
			result, err = repo.GetAttachment(ctx, *intent.AttachmentID)
			return err
		}
		if intent.Status != model.UploadStatusPending || time.Now().After(intent.ExpiresAt) {
			return ErrUploadExpired
		}
		meta, err := s.storage.GetObjectMetadata(ctx, &storage.GetObjectMetadataRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName})
		if err != nil {
			return err
		}
		maxSize := s.cfg.FileMaxBytes
		switch intent.MediaType {
		case model.AttachmentTypeImage:
			maxSize = s.cfg.ImageMaxBytes
		case model.AttachmentTypeVideo:
			maxSize = s.cfg.VideoMaxBytes
		}
		if meta.Size <= 0 || meta.Size > maxSize || meta.Size != intent.DeclaredSize || !strings.EqualFold(strings.Split(meta.ContentType, ";")[0], intent.ContentType) {
			return ErrInvalidAttachment
		}
		publicURL, err := s.storage.GetPublicURL(ctx, &storage.GetPublicURLRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName})
		if err != nil {
			return err
		}
		status := model.ProcessingStatusReady
		if intent.MediaType == model.AttachmentTypeVideo {
			status = model.ProcessingStatusProcessing
		}
		result = &model.PostAttachment{UserID: userID, URL: publicURL.Url, ObjectKey: intent.Folder + "/" + intent.FileName, MediaType: intent.MediaType, OriginalName: intent.OriginalName, ContentType: intent.ContentType, Size: meta.Size, ProcessingStatus: status}
		if err := repo.CreateAttachment(ctx, result); err != nil {
			return err
		}
		intent.Status, intent.AttachmentID = model.UploadStatusCompleted, &result.ID
		return repo.SaveUploadIntent(ctx, intent)
	})
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to complete upload")
	}
	return dto.ToAttachmentResponse(result), nil
}

func (s *Service) CancelUpload(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	return httpx.FromError(s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		intent, err := repo.GetUploadIntentForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if intent.UserID != userID {
			return ErrForbidden
		}
		if intent.Status != model.UploadStatusPending {
			return nil
		}
		if _, err := s.storage.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName}); err != nil {
			return err
		}
		intent.Status = model.UploadStatusCancelled
		return repo.SaveUploadIntent(ctx, intent)
	}), "[ERROR] Failed to cancel upload")
}

func (s *Service) CleanupExpiredUploads(ctx context.Context, limit int) (int, error) {
	intents, err := s.repo.ListExpiredUploadIntents(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	cleaned := 0
	for _, intent := range intents {
		if _, err := s.storage.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: strings.Split(intent.Folder, "/"), FileName: intent.FileName}); err != nil {
			continue
		}
		intent.Status = model.UploadStatusCancelled
		if err := s.repo.SaveUploadIntent(ctx, intent); err == nil {
			cleaned++
		}
	}
	orphans, err := s.repo.ListOrphanAttachments(ctx, time.Now().Add(-s.cfg.UploadIntentTTL), limit)
	if err != nil {
		return cleaned, err
	}
	for _, attachment := range orphans {
		parts := strings.Split(strings.Trim(attachment.ObjectKey, "/"), "/")
		if len(parts) < 2 {
			continue
		}
		if _, err := s.storage.DeleteObject(ctx, &storage.DeleteObjectRequest{Folders: parts[:len(parts)-1], FileName: parts[len(parts)-1]}); err != nil {
			continue
		}
		if err := s.repo.DeleteAttachmentPermanently(ctx, attachment); err == nil {
			cleaned++
		}
	}
	return cleaned, nil
}

func (s *Service) classroomAccess(ctx context.Context, classroomID uint, userID string) (*classrooms.GetClassroomPostAccessResponse, error) {
	access, err := s.classrooms.GetClassroomPostAccess(ctx, &classrooms.GetClassroomPostAccessRequest{
		ClassroomId: uint64(classroomID),
		UserId:      userID,
	})
	if err != nil {
		return nil, err
	}
	if !access.IsTeacher && !access.IsApprovedMember {
		return nil, ErrForbidden
	}
	return access, nil
}

func createInteractionOutbox(ctx context.Context, repo *repository.Repository, eventKey, recipient, title, body, dataType string, postID uint, commentID *uint, classroomID *uint) error {
	if recipient == "" {
		return nil
	}
	data := datatypes.JSONMap{"id": fmt.Sprint(postID), "type": dataType}
	if commentID != nil {
		data["commentId"] = fmt.Sprint(*commentID)
	}
	if classroomID != nil {
		data["classroomId"] = fmt.Sprint(*classroomID)
	}
	return repo.CreateOutbox(ctx, &model.NotificationOutbox{
		EventKey:      eventKey,
		Recipients:    datatypes.JSONSlice[string]{recipient},
		Title:         title,
		Body:          body,
		Data:          data,
		NextAttemptAt: time.Now(),
	})
}

func (s *Service) CreateCommunityPost(ctx context.Context, userID string, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, ErrPostEmpty
	}
	post := &model.Post{
		AuthorID:      userID,
		Scope:         model.PostScopeCommunity,
		Content:       req.Content,
		Status:        model.PostStatusPublished,
		AllowComments: req.CommentsAllowed(),
	}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.CreatePost(ctx, post); err != nil {
			return err
		}
		return s.attachToPost(ctx, repo, userID, post.ID, req.AttachmentIDs)
	}); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create community post")
	}
	return s.toPostResponse(ctx, post, userID, false), nil
}

func (s *Service) CreateClassroomPost(ctx context.Context, userID string, classroomID uint, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, ErrPostEmpty
	}
	access, err := s.classroomAccess(ctx, classroomID, userID)
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
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.CreatePost(ctx, post); err != nil {
			return err
		}
		if err := s.attachToPost(ctx, repo, userID, post.ID, req.AttachmentIDs); err != nil {
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
	return s.toPostResponse(ctx, post, userID, true), nil
}

func (s *Service) attachToPost(ctx context.Context, repo *repository.Repository, userID string, postID uint, ids []uint) error {
	if len(ids) > s.cfg.MaxAttachments {
		return ErrInvalidAttachment
	}
	if err := repo.AttachToPost(ctx, ids, userID, postID); err != nil {
		return ErrInvalidAttachment
	}
	attachments, err := repo.ListAttachments(ctx, postID)
	if err != nil {
		return err
	}
	var total int64
	videoCount := 0
	for _, attachment := range attachments {
		total += attachment.Size
		if attachment.MediaType == model.AttachmentTypeVideo {
			videoCount++
		}
	}
	if total > s.cfg.PostMaxBytes || videoCount > 1 {
		return ErrInvalidAttachment
	}
	return nil
}
func (s *Service) ListCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, model.PostStatusPublished, nil, q.PaginationQuery)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list community posts")
	}
	return s.toPostsResponse(ctx, items, userID, false, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) ListMyCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	posts, total, err := s.repo.ListCommunityPostsByAuthor(ctx, userID, q.PaginationQuery)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list user posts")
	}
	return s.toPostsResponse(ctx, posts, userID, false, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) ListClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, false)
}

func (s *Service) ListPinnedClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, true)
}

func (s *Service) listClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery, pinnedOnly bool) (*dto.PostsResponse, *httpx.HTTPError) {
	access, err := s.classroomAccess(ctx, classroomID, userID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list classroom posts")
	}
	q.Normalize()
	var pinned *bool
	if pinnedOnly {
		pinnedValue := true
		pinned = &pinnedValue
	}
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeClassroom, &classroomID, model.PostStatusPublished, pinned, q.PaginationQuery)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list classroom posts")
	}
	return s.toPostsResponse(ctx, items, userID, access.IsTeacher, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) GetPost(ctx context.Context, userID string, id uint) (*dto.PostResponse, *httpx.HTTPError) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post")
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to get post")
		}
		teacher = access.IsTeacher
	}
	if post.Status != model.PostStatusPublished && post.AuthorID != userID {
		return nil, ErrNotFound
	}
	return s.toPostResponse(ctx, post, userID, teacher), nil
}

func (s *Service) UpdatePost(ctx context.Context, userID string, id uint, req dto.UpdatePostRequest) (*dto.PostResponse, *httpx.HTTPError) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post")
	}
	if post.AuthorID != userID {
		return nil, ErrForbidden
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to update post")
		}
		if !access.Active {
			return nil, ErrClassroomReadOnly
		}
		teacher = access.IsTeacher
	}
	post.Content = req.Content
	if req.AllowComments != nil {
		post.AllowComments = *req.AllowComments
	}
	if err := s.repo.SavePost(ctx, post); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post")
	}
	return s.toPostResponse(ctx, post, userID, teacher), nil
}

func (s *Service) DeletePost(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to delete post")
	}
	if post.AuthorID != userID {
		return ErrForbidden
	}
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to delete post")
		}
		if !access.Active {
			return ErrClassroomReadOnly
		}
	}
	return httpx.FromError(s.repo.DeletePost(ctx, post), "[ERROR] Failed to delete post")
}

func (s *Service) PinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError {
	return s.setPostPinned(ctx, userID, classroomID, postID, true)
}

func (s *Service) UnpinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError {
	return s.setPostPinned(ctx, userID, classroomID, postID, false)
}

func (s *Service) setPostPinned(ctx context.Context, userID string, classroomID, postID uint, pinned bool) *httpx.HTTPError {
	access, err := s.classroomAccess(ctx, classroomID, userID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update post pin")
	}
	if !access.IsTeacher {
		return ErrForbidden
	}
	if !access.Active {
		return ErrClassroomReadOnly
	}
	return httpx.FromError(s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		post, err := repo.GetPostForUpdate(ctx, postID)
		if err != nil {
			return err
		}
		if post.ClassroomID == nil || *post.ClassroomID != classroomID {
			return gorm.ErrRecordNotFound
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
func (s *Service) LikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError {
	return s.setPostLiked(ctx, userID, postID, true)
}

func (s *Service) UnlikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError {
	return s.setPostLiked(ctx, userID, postID, false)
}

func (s *Service) setPostLiked(ctx context.Context, userID string, postID uint, liked bool) *httpx.HTTPError {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update post like")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return ErrNotFound
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update post like")
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return httpx.FromError(s.repo.Transaction(ctx, func(repo *repository.Repository) error {
			created, err := repo.LikePost(ctx, postID, userID)
			if err != nil || !created || p.AuthorID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("post:%d:liked-by:%s", p.ID, userID), p.AuthorID,
				"New like", "Someone liked your post.", "POST_LIKED", p.ID, nil, p.ClassroomID)
		}), "[ERROR] Failed to update post like")
	}
	return httpx.FromError(s.repo.UnlikePost(ctx, postID, userID), "[ERROR] Failed to update post like")
}
func (s *Service) ListPostComments(ctx context.Context, userID string, postID uint) (*dto.CommentsResponse, *httpx.HTTPError) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, ErrNotFound
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
		}
		teacher = access.IsTeacher
	}
	items, err := s.repo.ListComments(ctx, postID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list post comments")
	}
	out := make([]*dto.CommentResponse, 0, len(items))
	for _, c := range items {
		likes, liked, _ := s.repo.CommentStats(ctx, c.ID, userID)
		out = append(out, dto.ToCommentResponse(c, userID, teacher, likes, liked))
	}
	s.enrichComments(ctx, out)
	return &dto.CommentsResponse{Comments: out}, nil
}
func (s *Service) CreatePostComment(ctx context.Context, userID string, postID uint, req dto.CreateCommentRequest) (*dto.CommentResponse, *httpx.HTTPError) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, ErrNotFound
	}
	if !p.AllowComments {
		return nil, ErrCommentsDisabled
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
		}
		if !a.Active {
			return nil, ErrClassroomReadOnly
		}
	}
	if req.ParentID != nil {
		return nil, ErrInvalidReply
	}
	c := &model.PostComment{PostID: postID, UserID: userID, Content: req.Content}
	if err := s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		recipient := p.AuthorID
		dataType := "POST_COMMENTED"
		body := "Someone commented on your post."
		if req.ReplyToCommentID != nil {
			target, err := repo.GetComment(ctx, *req.ReplyToCommentID)
			if err != nil || target.PostID != postID {
				return ErrInvalidReply
			}
			if target.ParentID == nil {
				c.ParentID = &target.ID
			} else {
				c.ParentID = target.ParentID
			}
			c.ReplyToUserID = &target.UserID
			recipient = target.UserID
			dataType = "COMMENT_REPLIED"
			body = "Someone replied to your comment."
		}
		if err := repo.CreateComment(ctx, c); err != nil {
			return err
		}
		if recipient == userID {
			return nil
		}
		return createInteractionOutbox(ctx, repo,
			fmt.Sprintf("comment:%d:created", c.ID), recipient,
			"New comment", body, dataType, p.ID, &c.ID, p.ClassroomID)
	}); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to create post comment")
	}
	return dto.ToCommentResponse(c, userID, false, 0, false), nil
}
func (s *Service) UpdatePostComment(ctx context.Context, userID string, id uint, req dto.UpdateCommentRequest) (*dto.CommentResponse, *httpx.HTTPError) {
	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post comment")
	}
	if c.UserID != userID {
		return nil, ErrForbidden
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post comment")
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, httpx.FromError(err, "[ERROR] Failed to update post comment")
		}
		if !a.Active {
			return nil, ErrClassroomReadOnly
		}
	}
	c.Content = req.Content
	if err := s.repo.SaveComment(ctx, c); err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to update post comment")
	}
	return dto.ToCommentResponse(c, userID, false, 0, false), nil
}
func (s *Service) DeletePostComment(ctx context.Context, userID string, id uint) *httpx.HTTPError {
	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to delete post comment")
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to delete post comment")
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to delete post comment")
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
		teacher = a.IsTeacher
	}
	if c.UserID != userID && !teacher {
		return ErrForbidden
	}
	return httpx.FromError(s.repo.DeleteComment(ctx, c), "[ERROR] Failed to delete post comment")
}
func (s *Service) LikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError {
	return s.setCommentLiked(ctx, userID, commentID, true)
}

func (s *Service) UnlikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError {
	return s.setCommentLiked(ctx, userID, commentID, false)
}

func (s *Service) setCommentLiked(ctx context.Context, userID string, commentID uint, liked bool) *httpx.HTTPError {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update comment like")
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return httpx.FromError(err, "[ERROR] Failed to update comment like")
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return httpx.FromError(err, "[ERROR] Failed to update comment like")
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return httpx.FromError(s.repo.Transaction(ctx, func(repo *repository.Repository) error {
			created, err := repo.LikeComment(ctx, commentID, userID)
			if err != nil || !created || c.UserID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("comment:%d:liked-by:%s", c.ID, userID), c.UserID,
				"New like", "Someone liked your comment.", "COMMENT_LIKED", p.ID, &c.ID, p.ClassroomID)
		}), "[ERROR] Failed to update comment like")
	}
	return httpx.FromError(s.repo.UnlikeComment(ctx, commentID, userID), "[ERROR] Failed to update comment like")
}

func (s *Service) ListPostsForModeration(ctx context.Context, userID string, q dto.ModerationPostsQuery) (*dto.PostsResponse, *httpx.HTTPError) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, q.Status, nil, q.PaginationQuery)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to list posts for moderation")
	}
	return s.toPostsResponse(ctx, items, userID, false, total, int(q.Page), int(q.PageSize)), nil
}
func (s *Service) ModeratePost(ctx context.Context, admin string, id uint, action model.ModerationAction, reason string) *httpx.HTTPError {
	return httpx.FromError(s.repo.Transaction(ctx, func(r *repository.Repository) error {
		p, err := r.GetPostForUpdate(ctx, id)
		if err != nil {
			return err
		}
		switch action {
		case model.ModerationHide:
			p.Status = model.PostStatusHidden
		case model.ModerationUnhide:
			p.Status = model.PostStatusPublished
		case model.ModerationDelete:
			if err := r.CreateAudit(ctx, &model.PostModerationAudit{PostID: id, AdminUserID: admin, Action: action, Reason: reason}); err != nil {
				return err
			}
			return r.DeletePost(ctx, p)
		}
		if err := r.SavePost(ctx, p); err != nil {
			return err
		}
		return r.CreateAudit(ctx, &model.PostModerationAudit{PostID: id, AdminUserID: admin, Action: action, Reason: reason})
	}), "[ERROR] Failed to moderate post")
}
func (s *Service) GetPostModerationAudits(ctx context.Context, id uint) ([]*dto.AuditResponse, *httpx.HTTPError) {
	items, err := s.repo.ListAudits(ctx, id)
	if err != nil {
		return nil, httpx.FromError(err, "[ERROR] Failed to get post moderation audits")
	}
	return dto.ToAuditResponses(items), nil
}
func (s *Service) toPostResponse(ctx context.Context, p *model.Post, user string, teacher bool) *dto.PostResponse {
	likes, comments, liked, _ := s.repo.PostStats(ctx, p.ID, user)
	r := dto.ToPostResponse(p, user, teacher, likes, comments, liked, s.postAttachments(ctx, p.ID))
	s.enrichPosts(ctx, []*dto.PostResponse{r})
	return r
}
func (s *Service) toPostsResponse(ctx context.Context, items []*model.Post, user string, teacher bool, total int64, page, size int) *dto.PostsResponse {
	out := make([]*dto.PostResponse, len(items))
	for i, p := range items {
		likes, comments, liked, _ := s.repo.PostStats(ctx, p.ID, user)
		out[i] = dto.ToPostResponse(p, user, teacher, likes, comments, liked, s.postAttachments(ctx, p.ID))
	}
	s.enrichPosts(ctx, out)
	return dto.ToPostsResponse(out, total, page, size)
}
func (s *Service) postAttachments(ctx context.Context, postID uint) []*dto.AttachmentResponse {
	items, err := s.repo.ListAttachments(ctx, postID)
	if err != nil {
		return []*dto.AttachmentResponse{}
	}
	return dto.ToAttachmentResponses(items)
}
func (s *Service) enrichPosts(ctx context.Context, items []*dto.PostResponse) {
	ids := make([]string, 0, len(items))
	for _, v := range items {
		ids = append(ids, v.AuthorID)
	}
	m := s.getUsersByID(ctx, ids)
	for _, v := range items {
		v.Author = m[v.AuthorID]
	}
}
func (s *Service) enrichComments(ctx context.Context, items []*dto.CommentResponse) {
	ids := []string{}
	for _, v := range items {
		ids = append(ids, v.UserID)
		if v.ReplyToUserID != nil {
			ids = append(ids, *v.ReplyToUserID)
		}
	}
	m := s.getUsersByID(ctx, ids)
	for _, v := range items {
		v.User = m[v.UserID]
		if v.ReplyToUserID != nil {
			v.ReplyToUser = m[*v.ReplyToUserID]
		}
	}
}
func (s *Service) getUsersByID(ctx context.Context, ids []string) map[string]*dto.UserInfo {
	if len(ids) == 0 {
		return map[string]*dto.UserInfo{}
	}
	usersResp, err := s.identity.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: ids})
	if err != nil {
		return map[string]*dto.UserInfo{}
	}
	return dto.ToUserInfoMap(usersResp.Users)
}
func (s *Service) ProcessNotificationOutbox(ctx context.Context, limit int) (int, error) {
	if s.notifications == nil {
		return 0, nil
	}
	items, err := s.repo.ClaimOutbox(ctx, time.Now(), limit)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, v := range items {
		data := map[string]string{}
		for k, x := range v.Data {
			data[k] = fmt.Sprint(x)
		}
		_, e := s.notifications.CreateNotifications(ctx, &notifications.CreateNotificationsRequest{Recipients: []string(v.Recipients), Title: v.Title, Body: v.Body, SourceService: "POSTS", Data: data, IdempotencyKey: v.EventKey})
		if e != nil {
			attempts := v.Attempts + 1
			delay := time.Duration(math.Min(math.Pow(2, float64(attempts)), 300)) * time.Second
			if x := s.repo.MarkOutboxFailed(ctx, v.ID, attempts, time.Now().Add(delay), e.Error()); x != nil {
				return done, x
			}
			continue
		}
		if e = s.repo.MarkOutboxProcessed(ctx, v.ID, time.Now()); e != nil {
			return done, e
		}
		done++
	}
	return done, nil
}
