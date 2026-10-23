package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/service"
)

type Handler struct {
	service *service.Service
}

func New(postService *service.Service) *Handler {
	return &Handler{service: postService}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *jwt.AuthMiddleware, roleMiddleware *jwt.RoleMiddleware) {
	mux.HandleFunc("GET /posts", authMiddleware.Handle(h.ListCommunityPosts))
	mux.HandleFunc("GET /post-uploads/config", authMiddleware.Handle(h.GetUploadLimits))
	mux.HandleFunc("POST /post-uploads", authMiddleware.Handle(h.CreateUploadIntent))
	mux.HandleFunc("POST /post-uploads/{id}/complete", authMiddleware.Handle(h.CompleteUpload))
	mux.HandleFunc("DELETE /post-uploads/{id}", authMiddleware.Handle(h.CancelUpload))
	mux.HandleFunc("GET /posts/me", authMiddleware.Handle(h.ListMyCommunityPosts))
	mux.HandleFunc("POST /posts", authMiddleware.Handle(h.CreateCommunityPost))
	mux.HandleFunc("GET /posts/{id}", authMiddleware.Handle(h.GetPost))
	mux.HandleFunc("PATCH /posts/{id}", authMiddleware.Handle(h.UpdatePost))
	mux.HandleFunc("DELETE /posts/{id}", authMiddleware.Handle(h.DeletePost))
	mux.HandleFunc("GET /classrooms/{classroomId}/posts", authMiddleware.Handle(h.ListClassroomPosts))
	mux.HandleFunc("GET /classrooms/{classroomId}/posts/pinned", authMiddleware.Handle(h.ListPinnedClassroomPosts))
	mux.HandleFunc("POST /classrooms/{classroomId}/posts", authMiddleware.Handle(h.CreateClassroomPost))
	mux.HandleFunc("POST /classrooms/{classroomId}/posts/{id}/pin", authMiddleware.Handle(h.PinPost))
	mux.HandleFunc("DELETE /classrooms/{classroomId}/posts/{id}/pin", authMiddleware.Handle(h.UnpinPost))
	mux.HandleFunc("POST /posts/{id}/like", authMiddleware.Handle(h.LikePost))
	mux.HandleFunc("DELETE /posts/{id}/like", authMiddleware.Handle(h.UnlikePost))
	mux.HandleFunc("GET /posts/{id}/comments", authMiddleware.Handle(h.ListPostComments))
	mux.HandleFunc("POST /posts/{id}/comments", authMiddleware.Handle(h.CreatePostComment))
	mux.HandleFunc("PATCH /comments/{id}", authMiddleware.Handle(h.UpdatePostComment))
	mux.HandleFunc("DELETE /comments/{id}", authMiddleware.Handle(h.DeletePostComment))
	mux.HandleFunc("POST /comments/{id}/like", authMiddleware.Handle(h.LikePostComment))
	mux.HandleFunc("DELETE /comments/{id}/like", authMiddleware.Handle(h.UnlikePostComment))
	mux.HandleFunc("GET /admin/posts", authMiddleware.Handle(roleMiddleware.RequireRole("admin")(h.ListPostsForModeration)))
	mux.HandleFunc("POST /admin/posts/{id}/hide", authMiddleware.Handle(roleMiddleware.RequireRole("admin")(h.HidePost)))
	mux.HandleFunc("DELETE /admin/posts/{id}/hide", authMiddleware.Handle(roleMiddleware.RequireRole("admin")(h.UnhidePost)))
	mux.HandleFunc("DELETE /admin/posts/{id}", authMiddleware.Handle(roleMiddleware.RequireRole("admin")(h.DeletePostAsAdmin)))
	mux.HandleFunc("GET /admin/posts/{id}/audits", authMiddleware.Handle(roleMiddleware.RequireRole("admin")(h.GetPostModerationAudits)))
}

func (h *Handler) GetUploadLimits(w http.ResponseWriter, _ *http.Request) {
	httpx.JSON(w, http.StatusOK, h.service.UploadLimits())
}

func (h *Handler) CreateUploadIntent(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var request dto.CreateUploadIntentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreateUploadIntent(r.Context(), userID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	response, httpErr := h.service.CompleteUpload(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CancelUpload(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.CancelUpload(r.Context(), userID, id); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListCommunityPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	response, httpErr := h.service.ListCommunityPosts(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) ListMyCommunityPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	response, httpErr := h.service.ListMyCommunityPosts(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreateCommunityPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var request dto.CreatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreateCommunityPost(r.Context(), userID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) ListClassroomPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	response, httpErr := h.service.ListClassroomPosts(r.Context(), userID, classroomID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) ListPinnedClassroomPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	response, httpErr := h.service.ListPinnedClassroomPosts(r.Context(), userID, classroomID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreateClassroomPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.CreatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreateClassroomPost(r.Context(), userID, classroomID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	response, httpErr := h.service.GetPost(r.Context(), userID, postID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.UpdatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.UpdatePost(r.Context(), userID, postID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.DeletePost(r.Context(), userID, postID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PinPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.PinPost(r.Context(), userID, classroomID, postID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnpinPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.UnpinPost(r.Context(), userID, classroomID, postID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.LikePost(r.Context(), userID, postID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.UnlikePost(r.Context(), userID, postID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPostComments(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	response, httpErr := h.service.ListPostComments(r.Context(), userID, postID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreatePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.CreateCommentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.CreatePostComment(r.Context(), userID, postID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) UpdatePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.UpdateCommentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	response, httpErr := h.service.UpdatePostComment(r.Context(), userID, commentID, request)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) DeletePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.DeletePostComment(r.Context(), userID, commentID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LikePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.LikePostComment(r.Context(), userID, commentID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnlikePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	if httpErr := h.service.UnlikePostComment(r.Context(), userID, commentID); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPostsForModeration(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.ModerationPostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	response, httpErr := h.service.ListPostsForModeration(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) HidePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationHide, request.Reason); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnhidePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationUnhide, request.Reason); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeletePostAsAdmin(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if httpErr := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationDelete, request.Reason); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetPostModerationAudits(w http.ResponseWriter, r *http.Request) {
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	response, httpErr := h.service.GetPostModerationAudits(r.Context(), postID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
