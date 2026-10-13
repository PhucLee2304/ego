package handler

import (
	"errors"
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
	"ego/services/posts/internal/service"

	"gorm.io/gorm"
)

type Handler struct {
	service *service.Service
}

func New(postService *service.Service) *Handler {
	return &Handler{service: postService}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMiddleware *jwt.AuthMiddleware, roleMiddleware *jwt.RoleMiddleware) {
	mux.HandleFunc("GET /posts", authMiddleware.Handle(h.ListCommunityPosts))
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

func (h *Handler) ListCommunityPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	response, err := h.service.ListCommunityPosts(r.Context(), userID, query)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) ListMyCommunityPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	response, err := h.service.ListMyCommunityPosts(r.Context(), userID, query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreateCommunityPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var request dto.CreatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	response, err := h.service.CreateCommunityPost(r.Context(), userID, request)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) ListClassroomPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	response, err := h.service.ListClassroomPosts(r.Context(), userID, classroomID, query)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) ListPinnedClassroomPosts(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var query dto.PostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	response, err := h.service.ListPinnedClassroomPosts(r.Context(), userID, classroomID, query)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreateClassroomPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.CreatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	response, err := h.service.CreateClassroomPost(r.Context(), userID, classroomID, request)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	response, err := h.service.GetPost(r.Context(), userID, postID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.UpdatePostRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	response, err := h.service.UpdatePost(r.Context(), userID, postID, request)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.DeletePost(r.Context(), userID, postID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PinPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.PinPost(r.Context(), userID, classroomID, postID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnpinPost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	classroomID, err := httpx.ParsePathID(r, "classroomId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.UnpinPost(r.Context(), userID, classroomID, postID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.LikePost(r.Context(), userID, postID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.UnlikePost(r.Context(), userID, postID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPostComments(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	response, err := h.service.ListPostComments(r.Context(), userID, postID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) CreatePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.CreateCommentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	response, err := h.service.CreatePostComment(r.Context(), userID, postID, request)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusCreated, response)
}

func (h *Handler) UpdatePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.UpdateCommentRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	response, err := h.service.UpdatePostComment(r.Context(), userID, commentID, request)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) DeletePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.DeletePostComment(r.Context(), userID, commentID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) LikePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.LikePostComment(r.Context(), userID, commentID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnlikePostComment(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	commentID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	if err := h.service.UnlikePostComment(r.Context(), userID, commentID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListPostsForModeration(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	var query dto.ModerationPostsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	response, err := h.service.ListPostsForModeration(r.Context(), userID, query)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}

func (h *Handler) HidePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationHide, request.Reason); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UnhidePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationUnhide, request.Reason); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeletePostAsAdmin(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	var request dto.ModerationRequest
	if err := httpx.DecodeJSON(r, &request); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationDelete, request.Reason); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetPostModerationAudits(w http.ResponseWriter, r *http.Request) {
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid ID")
		return
	}
	response, err := h.service.GetPostModerationAudits(r.Context(), postID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Resource not found")
		case errors.Is(err, service.ErrForbidden):
			httpx.Error(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrClassroomReadOnly), errors.Is(err, service.ErrCommentsDisabled), errors.Is(err, service.ErrInvalidReply):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.JSON(w, http.StatusOK, response)
}
