package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"

	classrooms "ego/api/gen/go/classrooms"
	notifications "ego/api/gen/go/notifications"
	storage "ego/api/gen/go/storage"
	users "ego/api/gen/go/users"
	postsConfig "ego/services/posts/config"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrForbidden         = errors.New("[FORBIDDEN] You do not have permission")
	ErrClassroomReadOnly = errors.New("[ERROR] Archived classroom posts are read-only")
	ErrCommentsDisabled  = errors.New("[ERROR] Comments are disabled")
	ErrInvalidReply      = errors.New("[ERROR] Invalid reply target")
	ErrInvalidAttachment = errors.New("[ERROR] Invalid attachment")
	ErrUploadExpired     = errors.New("[ERROR] Upload has expired")
	ErrPostEmpty         = errors.New("[ERROR] Post content or attachment is required")
)

type Service struct {
	repo          *repository.Repository
	classrooms    classrooms.ClassroomServiceClient
	users         users.UserServiceClient
	notifications notifications.NotificationServiceClient
	storage       storage.StorageServiceClient
	cfg           *postsConfig.AppConfig
}

func New(
	repo *repository.Repository,
	appConfig *postsConfig.AppConfig,
	classroomsClient classrooms.ClassroomServiceClient,
	usersClient users.UserServiceClient,
	notificationsClient notifications.NotificationServiceClient,
	storageClient storage.StorageServiceClient,
) *Service {
	return &Service{
		repo:          repo,
		classrooms:    classroomsClient,
		users:         usersClient,
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

func (s *Service) CreateUploadIntent(ctx context.Context, userID string, req dto.CreateUploadIntentRequest) (*dto.UploadIntentResponse, error) {
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
		return nil, err
	}
	intent := &model.PostUploadIntent{UserID: userID, Folder: folder, FileName: fileName, OriginalName: req.OriginalName, ContentType: contentType, DeclaredSize: req.Size, MediaType: mediaType, Status: model.UploadStatusPending, ExpiresAt: time.Now().Add(s.cfg.UploadIntentTTL)}
	if err := s.repo.CreateUploadIntent(ctx, intent); err != nil {
		return nil, err
	}
	return &dto.UploadIntentResponse{ID: intent.ID, UploadURL: response.Url, ContentType: contentType, ExpiresAt: intent.ExpiresAt}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, userID string, id uint) (*dto.AttachmentResponse, error) {
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
		if intent.MediaType == model.AttachmentTypeImage {
			maxSize = s.cfg.ImageMaxBytes
		} else if intent.MediaType == model.AttachmentTypeVideo {
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
		return nil, err
	}
	return dto.ToAttachmentResponse(result), nil
}

func (s *Service) CancelUpload(ctx context.Context, userID string, id uint) error {
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
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
	})
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

func (s *Service) CreateCommunityPost(ctx context.Context, userID string, req dto.CreatePostRequest) (*dto.PostResponse, error) {
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
		return nil, err
	}
	return s.toPostResponse(ctx, post, userID, false), nil
}

func (s *Service) CreateClassroomPost(ctx context.Context, userID string, classroomID uint, req dto.CreatePostRequest) (*dto.PostResponse, error) {
	if strings.TrimSpace(req.Content) == "" && len(req.AttachmentIDs) == 0 {
		return nil, ErrPostEmpty
	}
	access, err := s.classroomAccess(ctx, classroomID, userID)
	if err != nil {
		return nil, err
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
		return nil, err
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
func (s *Service) ListCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, error) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, model.PostStatusPublished, nil, q.PaginationQuery)
	if err != nil {
		return nil, err
	}
	return s.toPostsResponse(ctx, items, userID, false, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) ListMyCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, error) {
	q.Normalize()
	posts, total, err := s.repo.ListCommunityPostsByAuthor(ctx, userID, q.PaginationQuery)
	if err != nil {
		return nil, err
	}
	return s.toPostsResponse(ctx, posts, userID, false, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) ListClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, error) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, false)
}

func (s *Service) ListPinnedClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, error) {
	return s.listClassroomPosts(ctx, userID, classroomID, q, true)
}

func (s *Service) listClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery, pinnedOnly bool) (*dto.PostsResponse, error) {
	access, err := s.classroomAccess(ctx, classroomID, userID)
	if err != nil {
		return nil, err
	}
	q.Normalize()
	var pinned *bool
	if pinnedOnly {
		pinnedValue := true
		pinned = &pinnedValue
	}
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeClassroom, &classroomID, model.PostStatusPublished, pinned, q.PaginationQuery)
	if err != nil {
		return nil, err
	}
	return s.toPostsResponse(ctx, items, userID, access.IsTeacher, total, int(q.Page), int(q.PageSize)), nil
}

func (s *Service) GetPost(ctx context.Context, userID string, id uint) (*dto.PostResponse, error) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, err
		}
		teacher = access.IsTeacher
	}
	if post.Status != model.PostStatusPublished && post.AuthorID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	return s.toPostResponse(ctx, post, userID, teacher), nil
}

func (s *Service) UpdatePost(ctx context.Context, userID string, id uint, req dto.UpdatePostRequest) (*dto.PostResponse, error) {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return nil, err
	}
	if post.AuthorID != userID {
		return nil, ErrForbidden
	}
	teacher := false
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return s.toPostResponse(ctx, post, userID, teacher), nil
}

func (s *Service) DeletePost(ctx context.Context, userID string, id uint) error {
	post, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return err
	}
	if post.AuthorID != userID {
		return ErrForbidden
	}
	if post.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *post.ClassroomID, userID)
		if err != nil {
			return err
		}
		if !access.Active {
			return ErrClassroomReadOnly
		}
	}
	return s.repo.DeletePost(ctx, post)
}

func (s *Service) PinPost(ctx context.Context, userID string, classroomID, postID uint) error {
	return s.setPostPinned(ctx, userID, classroomID, postID, true)
}

func (s *Service) UnpinPost(ctx context.Context, userID string, classroomID, postID uint) error {
	return s.setPostPinned(ctx, userID, classroomID, postID, false)
}

func (s *Service) setPostPinned(ctx context.Context, userID string, classroomID, postID uint, pinned bool) error {
	access, err := s.classroomAccess(ctx, classroomID, userID)
	if err != nil {
		return err
	}
	if !access.IsTeacher {
		return ErrForbidden
	}
	if !access.Active {
		return ErrClassroomReadOnly
	}
	return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
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
	})
}
func (s *Service) LikePost(ctx context.Context, userID string, postID uint) error {
	return s.setPostLiked(ctx, userID, postID, true)
}

func (s *Service) UnlikePost(ctx context.Context, userID string, postID uint) error {
	return s.setPostLiked(ctx, userID, postID, false)
}

func (s *Service) setPostLiked(ctx context.Context, userID string, postID uint, liked bool) error {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return err
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return gorm.ErrRecordNotFound
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return err
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
			created, err := repo.LikePost(ctx, postID, userID)
			if err != nil || !created || p.AuthorID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("post:%d:liked-by:%s", p.ID, userID), p.AuthorID,
				"New like", "Someone liked your post.", "POST_LIKED", p.ID, nil, p.ClassroomID)
		})
	}
	return s.repo.UnlikePost(ctx, postID, userID)
}
func (s *Service) ListPostComments(ctx context.Context, userID string, postID uint) (*dto.CommentsResponse, error) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		access, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, err
		}
		teacher = access.IsTeacher
	}
	items, err := s.repo.ListComments(ctx, postID)
	if err != nil {
		return nil, err
	}
	out := make([]*dto.CommentResponse, 0, len(items))
	for _, c := range items {
		likes, liked, _ := s.repo.CommentStats(ctx, c.ID, userID)
		out = append(out, dto.ToCommentResponse(c, userID, teacher, likes, liked))
	}
	s.enrichComments(ctx, out)
	return &dto.CommentsResponse{Comments: out}, nil
}
func (s *Service) CreatePostComment(ctx context.Context, userID string, postID uint, req dto.CreateCommentRequest) (*dto.CommentResponse, error) {
	p, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if p.Status != model.PostStatusPublished && p.AuthorID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	if !p.AllowComments {
		return nil, ErrCommentsDisabled
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	return dto.ToCommentResponse(c, userID, false, 0, false), nil
}
func (s *Service) UpdatePostComment(ctx context.Context, userID string, id uint, req dto.UpdateCommentRequest) (*dto.CommentResponse, error) {
	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.UserID != userID {
		return nil, ErrForbidden
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return nil, err
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return nil, err
		}
		if !a.Active {
			return nil, ErrClassroomReadOnly
		}
	}
	c.Content = req.Content
	if err := s.repo.SaveComment(ctx, c); err != nil {
		return nil, err
	}
	return dto.ToCommentResponse(c, userID, false, 0, false), nil
}
func (s *Service) DeletePostComment(ctx context.Context, userID string, id uint) error {
	c, err := s.repo.GetComment(ctx, id)
	if err != nil {
		return err
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return err
	}
	teacher := false
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return err
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
		teacher = a.IsTeacher
	}
	if c.UserID != userID && !teacher {
		return ErrForbidden
	}
	return s.repo.DeleteComment(ctx, c)
}
func (s *Service) LikePostComment(ctx context.Context, userID string, commentID uint) error {
	return s.setCommentLiked(ctx, userID, commentID, true)
}

func (s *Service) UnlikePostComment(ctx context.Context, userID string, commentID uint) error {
	return s.setCommentLiked(ctx, userID, commentID, false)
}

func (s *Service) setCommentLiked(ctx context.Context, userID string, commentID uint, liked bool) error {
	c, err := s.repo.GetComment(ctx, commentID)
	if err != nil {
		return err
	}
	p, err := s.repo.GetPost(ctx, c.PostID)
	if err != nil {
		return err
	}
	if p.Scope == model.PostScopeClassroom {
		a, err := s.classroomAccess(ctx, *p.ClassroomID, userID)
		if err != nil {
			return err
		}
		if !a.Active {
			return ErrClassroomReadOnly
		}
	}
	if liked {
		return s.repo.Transaction(ctx, func(repo *repository.Repository) error {
			created, err := repo.LikeComment(ctx, commentID, userID)
			if err != nil || !created || c.UserID == userID {
				return err
			}
			return createInteractionOutbox(ctx, repo,
				fmt.Sprintf("comment:%d:liked-by:%s", c.ID, userID), c.UserID,
				"New like", "Someone liked your comment.", "COMMENT_LIKED", p.ID, &c.ID, p.ClassroomID)
		})
	}
	return s.repo.UnlikeComment(ctx, commentID, userID)
}

func (s *Service) ListPostsForModeration(ctx context.Context, userID string, q dto.ModerationPostsQuery) (*dto.PostsResponse, error) {
	q.Normalize()
	items, total, err := s.repo.ListPosts(ctx, model.PostScopeCommunity, nil, q.Status, nil, q.PaginationQuery)
	if err != nil {
		return nil, err
	}
	return s.toPostsResponse(ctx, items, userID, false, total, int(q.Page), int(q.PageSize)), nil
}
func (s *Service) ModeratePost(ctx context.Context, admin string, id uint, action model.ModerationAction, reason string) error {
	return s.repo.Transaction(ctx, func(r *repository.Repository) error {
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
	})
}
func (s *Service) GetPostModerationAudits(ctx context.Context, id uint) ([]*dto.AuditResponse, error) {
	items, err := s.repo.ListAudits(ctx, id)
	if err != nil {
		return nil, err
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
	r, err := s.users.GetUsersInfo(ctx, &users.GetUsersInfoRequest{Ids: ids})
	if err != nil {
		return map[string]*dto.UserInfo{}
	}
	return dto.ToUserInfoMap(r.Users)
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
