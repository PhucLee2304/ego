package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	identityClient "ego/api/gen/go/identity"
	learning "ego/api/gen/go/learning"
	notifications "ego/api/gen/go/notifications"
	storage "ego/api/gen/go/storage"
	"ego/platform/httpx"
	postsConfig "ego/services/posts/config"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"

	"google.golang.org/grpc"
	"gorm.io/datatypes"
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

type Service interface {
	// Upload
	UploadLimits() *dto.UploadLimitsResponse
	CreateUploadIntent(ctx context.Context, userID string, req dto.CreateUploadIntentRequest) (*dto.UploadIntentResponse, *httpx.HTTPError)
	CompleteUpload(ctx context.Context, userID string, id uint) (*dto.AttachmentResponse, *httpx.HTTPError)
	CancelUpload(ctx context.Context, userID string, id uint) *httpx.HTTPError

	// Post
	CreateCommunityPost(ctx context.Context, userID string, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError)
	CreateClassroomPost(ctx context.Context, userID string, classroomID uint, req dto.CreatePostRequest) (*dto.PostResponse, *httpx.HTTPError)
	ListCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError)
	ListMyCommunityPosts(ctx context.Context, userID string, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError)
	ListClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError)
	ListPinnedClassroomPosts(ctx context.Context, userID string, classroomID uint, q dto.PostsQuery) (*dto.PostsResponse, *httpx.HTTPError)
	GetPost(ctx context.Context, userID string, id uint) (*dto.PostResponse, *httpx.HTTPError)
	UpdatePost(ctx context.Context, userID string, id uint, req dto.UpdatePostRequest) (*dto.PostResponse, *httpx.HTTPError)
	DeletePost(ctx context.Context, userID string, id uint) *httpx.HTTPError
	PinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError
	UnpinPost(ctx context.Context, userID string, classroomID, postID uint) *httpx.HTTPError

	// Comment
	ListPostComments(ctx context.Context, userID string, postID uint, query dto.CommentsQuery) (*dto.CommentsResponse, *httpx.HTTPError)
	GetPostComment(ctx context.Context, userID string, id uint) (*dto.CommentResponse, *httpx.HTTPError)
	CreatePostComment(ctx context.Context, userID string, postID uint, req dto.CreateCommentRequest) (*dto.CommentResponse, *httpx.HTTPError)

	// Like
	LikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError
	UnlikePost(ctx context.Context, userID string, postID uint) *httpx.HTTPError
	LikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError
	UnlikePostComment(ctx context.Context, userID string, commentID uint) *httpx.HTTPError

	// Moderation (admin)
	ListPostsForModeration(ctx context.Context, userID string, q dto.ModerationPostsQuery) (*dto.PostsResponse, *httpx.HTTPError)
	ModeratePost(ctx context.Context, adminID string, id uint, action model.ModerationAction, reason string) *httpx.HTTPError
	GetPostModerationAudits(ctx context.Context, id uint) ([]*dto.AuditResponse, *httpx.HTTPError)

	// Stats (admin)
	GetAdminStats(ctx context.Context, query dto.AdminStatsQuery) (*dto.AdminStatsResponse, *httpx.HTTPError)

	// Worker
	ProcessNotificationOutbox(ctx context.Context, limit int) (int, error)
	CleanupExpiredUploads(ctx context.Context, limit int) (int, error)
	ProcessVideos(ctx context.Context, limit int) error
}

type LearningClient interface {
	GetClassroomPostAccess(ctx context.Context, in *learning.GetClassroomPostAccessRequest, opts ...grpc.CallOption) (*learning.GetClassroomPostAccessResponse, error)
}

type IdentityClient interface {
	GetUsersInfo(ctx context.Context, in *identityClient.GetUsersInfoRequest, opts ...grpc.CallOption) (*identityClient.GetUsersInfoResponse, error)
}

type NotificationClient interface {
	CreateNotifications(ctx context.Context, in *notifications.CreateNotificationsRequest, opts ...grpc.CallOption) (*notifications.CreateNotificationsResponse, error)
}

type StorageClient interface {
	GeneratePresignedUploadURL(ctx context.Context, in *storage.GeneratePresignedUploadURLRequest, opts ...grpc.CallOption) (*storage.GeneratePresignedUploadURLResponse, error)
	GetPublicURL(ctx context.Context, in *storage.GetPublicURLRequest, opts ...grpc.CallOption) (*storage.GetPublicURLResponse, error)
	GetObjectMetadata(ctx context.Context, in *storage.GetObjectMetadataRequest, opts ...grpc.CallOption) (*storage.GetObjectMetadataResponse, error)
	DeleteObject(ctx context.Context, in *storage.DeleteObjectRequest, opts ...grpc.CallOption) (*storage.DeleteObjectResponse, error)
}

type service struct {
	repo               repository.Repository
	learningClient     LearningClient
	identityClient     IdentityClient
	notificationClient NotificationClient
	storageClient      StorageClient
	cfg                *postsConfig.AppConfig
}

func New(
	repo repository.Repository,
	appConfig *postsConfig.AppConfig,
	learningClient LearningClient,
	identityClient IdentityClient,
	notificationClient NotificationClient,
	storageClient StorageClient,
) Service {
	return &service{
		repo:               repo,
		learningClient:     learningClient,
		identityClient:     identityClient,
		notificationClient: notificationClient,
		storageClient:      storageClient,
		cfg:                appConfig,
	}
}

// ensureBelongsToClassroom kiểm tra người dùng thuộc lớp (giáo viên hoặc học viên đã duyệt), không thì ErrForbidden;
// trả về thông tin quyền để nơi gọi kiểm tra thêm IsTeacher/Active khi cần.
func (s *service) ensureBelongsToClassroom(ctx context.Context, classroomID uint, userID string) (*learning.GetClassroomPostAccessResponse, error) {
	access, err := s.learningClient.GetClassroomPostAccess(ctx, &learning.GetClassroomPostAccessRequest{
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

func createInteractionOutbox(ctx context.Context, repo repository.Repository, eventKey, recipient, title, body, dataType string, postID uint, commentID *uint, classroomID *uint) error {
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

// hydratePosts lấy số liệu, đính kèm và tác giả của cả trang bằng vài truy vấn gộp, rồi để dto ghép response.
func (s *service) hydratePosts(ctx context.Context, items []*model.Post, userID string, teacher bool) ([]*dto.PostResponse, error) {
	ids := make([]uint, len(items))
	authorIDs := make([]string, len(items))
	for i, p := range items {
		ids[i], authorIDs[i] = p.ID, p.AuthorID
	}
	stats, err := s.repo.GetPostStats(ctx, ids, userID)
	if err != nil {
		return nil, err
	}
	attachments, err := s.repo.ListAttachments(ctx, ids)
	if err != nil {
		return nil, err
	}
	return dto.ToPostResponses(items, userID, teacher, stats, attachments, s.getUsers(ctx, authorIDs)), nil
}

// hydrateComments lấy số like/reply và người dùng của cả trang bằng truy vấn gộp, rồi để dto ghép response.
func (s *service) hydrateComments(ctx context.Context, items []*model.PostComment, userID string, teacher bool) ([]*dto.CommentResponse, error) {
	ids := make([]uint, len(items))
	userIDs := make([]string, 0, len(items))
	for i, c := range items {
		ids[i] = c.ID
		userIDs = append(userIDs, c.UserID)
		if c.ReplyToUserID != nil {
			userIDs = append(userIDs, *c.ReplyToUserID)
		}
	}
	stats, err := s.repo.GetCommentStats(ctx, ids, userID)
	if err != nil {
		return nil, err
	}
	return dto.ToCommentResponses(items, userID, teacher, stats, s.getUsers(ctx, userIDs)), nil
}

// getUsers lấy thông tin người dùng để hiển thị; identity lỗi thì trả rỗng, client vẫn có userId.
func (s *service) getUsers(ctx context.Context, ids []string) []*identityClient.UserInfo {
	if len(ids) == 0 {
		return nil
	}
	usersResp, err := s.identityClient.GetUsersInfo(ctx, &identityClient.GetUsersInfoRequest{Ids: ids})
	if err != nil {
		return nil
	}
	return usersResp.Users
}
