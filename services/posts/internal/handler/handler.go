package handler

import (
	"net/http"

	"ego/platform/jwt"
	"ego/services/posts/internal/service"
)

// Handler của từng domain được cài đặt ở <domain>.go và đăng ký trong RegisterRoutes.
type Handler interface {
	RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware)

	// Upload
	GetUploadLimits(w http.ResponseWriter, r *http.Request)
	CreateUploadIntent(w http.ResponseWriter, r *http.Request)
	CompleteUpload(w http.ResponseWriter, r *http.Request)
	CancelUpload(w http.ResponseWriter, r *http.Request)

	// Post
	ListCommunityPosts(w http.ResponseWriter, r *http.Request)
	ListMyCommunityPosts(w http.ResponseWriter, r *http.Request)
	CreateCommunityPost(w http.ResponseWriter, r *http.Request)
	ListClassroomPosts(w http.ResponseWriter, r *http.Request)
	ListPinnedClassroomPosts(w http.ResponseWriter, r *http.Request)
	CreateClassroomPost(w http.ResponseWriter, r *http.Request)
	GetPost(w http.ResponseWriter, r *http.Request)
	UpdatePost(w http.ResponseWriter, r *http.Request)
	DeletePost(w http.ResponseWriter, r *http.Request)
	PinPost(w http.ResponseWriter, r *http.Request)
	UnpinPost(w http.ResponseWriter, r *http.Request)

	// Comment
	ListPostComments(w http.ResponseWriter, r *http.Request)
	GetPostComment(w http.ResponseWriter, r *http.Request)
	CreatePostComment(w http.ResponseWriter, r *http.Request)

	// Like
	LikePost(w http.ResponseWriter, r *http.Request)
	UnlikePost(w http.ResponseWriter, r *http.Request)
	LikePostComment(w http.ResponseWriter, r *http.Request)
	UnlikePostComment(w http.ResponseWriter, r *http.Request)

	// Moderation (admin)
	ListPostsForModeration(w http.ResponseWriter, r *http.Request)
	HidePost(w http.ResponseWriter, r *http.Request)
	UnhidePost(w http.ResponseWriter, r *http.Request)
	DeletePostAsAdmin(w http.ResponseWriter, r *http.Request)
	GetPostModerationAudits(w http.ResponseWriter, r *http.Request)

	// Stats (admin)
	GetAdminStats(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(s service.Service) Handler {
	return &handler{service: s}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware, rm *jwt.RoleMiddleware) {
	// Upload
	mux.HandleFunc("GET /uploads/config", mw.Handle(h.GetUploadLimits))
	mux.HandleFunc("POST /uploads", mw.Handle(h.CreateUploadIntent))
	mux.HandleFunc("POST /uploads/{id}/complete", mw.Handle(h.CompleteUpload))
	mux.HandleFunc("DELETE /uploads/{id}", mw.Handle(h.CancelUpload))

	// Post
	mux.HandleFunc("GET /posts", mw.Handle(h.ListCommunityPosts))
	mux.HandleFunc("GET /posts/me", mw.Handle(h.ListMyCommunityPosts))
	mux.HandleFunc("POST /posts", mw.Handle(h.CreateCommunityPost))
	mux.HandleFunc("GET /posts/{id}", mw.Handle(h.GetPost))
	mux.HandleFunc("PATCH /posts/{id}", mw.Handle(h.UpdatePost))
	mux.HandleFunc("DELETE /posts/{id}", mw.Handle(h.DeletePost))
	mux.HandleFunc("GET /classrooms/{classroomId}/posts", mw.Handle(h.ListClassroomPosts))
	mux.HandleFunc("GET /classrooms/{classroomId}/posts/pinned", mw.Handle(h.ListPinnedClassroomPosts))
	mux.HandleFunc("POST /classrooms/{classroomId}/posts", mw.Handle(h.CreateClassroomPost))
	mux.HandleFunc("POST /classrooms/{classroomId}/posts/{id}/pin", mw.Handle(h.PinPost))
	mux.HandleFunc("DELETE /classrooms/{classroomId}/posts/{id}/pin", mw.Handle(h.UnpinPost))

	// Comment
	mux.HandleFunc("GET /posts/{id}/comments", mw.Handle(h.ListPostComments))
	mux.HandleFunc("POST /posts/{id}/comments", mw.Handle(h.CreatePostComment))
	mux.HandleFunc("GET /comments/{id}", mw.Handle(h.GetPostComment))

	// Like
	mux.HandleFunc("POST /posts/{id}/like", mw.Handle(h.LikePost))
	mux.HandleFunc("DELETE /posts/{id}/like", mw.Handle(h.UnlikePost))
	mux.HandleFunc("POST /comments/{id}/like", mw.Handle(h.LikePostComment))
	mux.HandleFunc("DELETE /comments/{id}/like", mw.Handle(h.UnlikePostComment))

	// Moderation (admin)
	mux.HandleFunc("GET /admin/posts", mw.Handle(rm.RequireRole("admin")(h.ListPostsForModeration)))              // ** Not used**
	mux.HandleFunc("PUT /admin/posts/{id}/hide", mw.Handle(rm.RequireRole("admin")(h.HidePost)))                  // ** Not used**
	mux.HandleFunc("DELETE /admin/posts/{id}/hide", mw.Handle(rm.RequireRole("admin")(h.UnhidePost)))             // ** Not used**
	mux.HandleFunc("DELETE /admin/posts/{id}", mw.Handle(rm.RequireRole("admin")(h.DeletePostAsAdmin)))           // ** Not used**
	mux.HandleFunc("GET /admin/posts/{id}/audits", mw.Handle(rm.RequireRole("admin")(h.GetPostModerationAudits))) // ** Not used**

	// Stats (admin)
	mux.HandleFunc("GET /admin/stats", mw.Handle(rm.RequireRole("admin")(h.GetAdminStats)))
}
