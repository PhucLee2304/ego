package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	classrooms "ego/api/gen/go/classrooms"
	notifications "ego/api/gen/go/notifications"
	users "ego/api/gen/go/users"
	"ego/platform/httpx"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/repository"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	ErrForbidden         = errors.New("[FORBIDDEN] You do not have permission")
	ErrClassroomReadOnly = errors.New("[ERROR] Archived classroom posts are read-only")
	ErrCommentsDisabled  = errors.New("[ERROR] Comments are disabled")
	ErrInvalidReply      = errors.New("[ERROR] Invalid reply target")
)

type Service struct {
	repo          *repository.Repository
	classrooms    classrooms.ClassroomServiceClient
	users         users.UserServiceClient
	notifications notifications.NotificationServiceClient
}

func New(repo *repository.Repository, classroomsClient classrooms.ClassroomServiceClient, usersClient users.UserServiceClient, notificationsClient notifications.NotificationServiceClient) *Service {
	return &Service{
		repo:          repo,
		classrooms:    classroomsClient,
		users:         usersClient,
		notifications: notificationsClient,
	}
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

func allowComments(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
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
	post := &model.Post{
		AuthorID:      userID,
		Scope:         model.PostScopeCommunity,
		Content:       req.Content,
		Status:        model.PostStatusPublished,
		AllowComments: allowComments(req.AllowComments),
	}
	if err := s.repo.CreatePost(ctx, post); err != nil {
		return nil, err
	}
	return s.toPostResponse(ctx, post, userID, false), nil
}

func (s *Service) CreateClassroomPost(ctx context.Context, userID string, classroomID uint, req dto.CreatePostRequest) (*dto.PostResponse, error) {
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
		AllowComments: allowComments(req.AllowComments),
	}
	err = s.repo.Transaction(ctx, func(repo *repository.Repository) error {
		if err := repo.CreatePost(ctx, post); err != nil {
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
		out = append(out, &dto.CommentResponse{ID: c.ID, PostID: c.PostID, UserID: c.UserID, Content: c.Content, ParentID: c.ParentID, ReplyToUserID: c.ReplyToUserID, LikeCount: likes, Liked: liked, CanEdit: c.UserID == userID, CanDelete: c.UserID == userID || teacher, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt})
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
	return &dto.CommentResponse{
		ID:            c.ID,
		PostID:        c.PostID,
		UserID:        c.UserID,
		Content:       c.Content,
		ParentID:      c.ParentID,
		ReplyToUserID: c.ReplyToUserID,
		CanEdit:       true,
		CanDelete:     true,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}, nil
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
	return &dto.CommentResponse{ID: c.ID, PostID: c.PostID, UserID: c.UserID, Content: c.Content, ParentID: c.ParentID, ReplyToUserID: c.ReplyToUserID, CanEdit: true, CanDelete: true, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}, nil
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
	out := make([]*dto.AuditResponse, len(items))
	for i, a := range items {
		out[i] = &dto.AuditResponse{ID: a.ID, PostID: a.PostID, AdminUserID: a.AdminUserID, Action: a.Action, Reason: a.Reason, CreatedAt: a.CreatedAt}
	}
	return out, nil
}
func (s *Service) toPostResponse(ctx context.Context, p *model.Post, user string, teacher bool) *dto.PostResponse {
	likes, comments, liked, _ := s.repo.PostStats(ctx, p.ID, user)
	r := &dto.PostResponse{ID: p.ID, AuthorID: p.AuthorID, Scope: p.Scope, ClassroomID: p.ClassroomID, Content: p.Content, Status: p.Status, Pinned: p.Pinned, AllowComments: p.AllowComments, LikeCount: likes, CommentCount: comments, Liked: liked, CanEdit: p.AuthorID == user, CanDelete: p.AuthorID == user, CanPin: teacher, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	s.enrichPosts(ctx, []*dto.PostResponse{r})
	return r
}
func (s *Service) toPostsResponse(ctx context.Context, items []*model.Post, user string, teacher bool, total int64, page, size int) *dto.PostsResponse {
	out := make([]*dto.PostResponse, len(items))
	for i, p := range items {
		likes, comments, liked, _ := s.repo.PostStats(ctx, p.ID, user)
		out[i] = &dto.PostResponse{ID: p.ID, AuthorID: p.AuthorID, Scope: p.Scope, ClassroomID: p.ClassroomID, Content: p.Content, Status: p.Status, Pinned: p.Pinned, AllowComments: p.AllowComments, LikeCount: likes, CommentCount: comments, Liked: liked, CanEdit: p.AuthorID == user, CanDelete: p.AuthorID == user, CanPin: teacher, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	}
	s.enrichPosts(ctx, out)
	return &dto.PostsResponse{Posts: out, Page: page, PageCount: httpx.ToPageCounts(total, int32(size))}
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
	out := map[string]*dto.UserInfo{}
	if len(ids) == 0 {
		return out
	}
	r, err := s.users.GetUsersInfo(ctx, &users.GetUsersInfoRequest{Ids: ids})
	if err != nil {
		return out
	}
	for _, u := range r.Users {
		var avatar *string
		if u.Avatar != "" {
			avatar = &u.Avatar
		}
		out[u.Id] = &dto.UserInfo{ID: u.Id, Name: u.Name, Email: u.Email, Avatar: avatar}
	}
	return out
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
