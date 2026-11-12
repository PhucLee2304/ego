package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/posts/internal/dto"
)

// ListCommunityPosts godoc
// @Summary      List community posts
// @Description  Published community posts, newest first
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200  {object}  dto.PostsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts [get]
func (h *handler) ListCommunityPosts(w http.ResponseWriter, r *http.Request) {
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

// ListMyCommunityPosts godoc
// @Summary      List my community posts
// @Description  Community posts of the current user, including hidden ones
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200  {object}  dto.PostsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts/me [get]
func (h *handler) ListMyCommunityPosts(w http.ResponseWriter, r *http.Request) {
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

// CreateCommunityPost godoc
// @Summary      Create a community post
// @Description  Content or at least one completed attachment is required
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body dto.CreatePostRequest true "Post"
// @Success      201  {object}  dto.PostResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts [post]
func (h *handler) CreateCommunityPost(w http.ResponseWriter, r *http.Request) {
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

// ListClassroomPosts godoc
// @Summary      List classroom posts
// @Description  Published posts of a classroom; teacher or approved member only
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        classroomId path int true "Classroom ID"
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200  {object}  dto.PostsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /classrooms/{classroomId}/posts [get]
func (h *handler) ListClassroomPosts(w http.ResponseWriter, r *http.Request) {
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

// ListPinnedClassroomPosts godoc
// @Summary      List pinned classroom posts
// @Description  Pinned posts of a classroom; teacher or approved member only
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        classroomId path int true "Classroom ID"
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200  {object}  dto.PostsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /classrooms/{classroomId}/posts/pinned [get]
func (h *handler) ListPinnedClassroomPosts(w http.ResponseWriter, r *http.Request) {
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

// CreateClassroomPost godoc
// @Summary      Create a classroom post
// @Description  Teacher only; notifies approved students
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        classroomId path int true "Classroom ID"
// @Param        request body dto.CreatePostRequest true "Post"
// @Success      201  {object}  dto.PostResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /classrooms/{classroomId}/posts [post]
func (h *handler) CreateClassroomPost(w http.ResponseWriter, r *http.Request) {
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

// GetPost godoc
// @Summary      Get a post
// @Description  Hidden posts are visible to their author only
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Success      200  {object}  dto.PostResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts/{id} [get]
func (h *handler) GetPost(w http.ResponseWriter, r *http.Request) {
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

// UpdatePost godoc
// @Summary      Update a post
// @Description  Author only; omitted fields are kept, attachmentIds replaces the attachment list
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Param        request body dto.UpdatePostRequest true "Fields to update"
// @Success      200  {object}  dto.PostResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts/{id} [patch]
func (h *handler) UpdatePost(w http.ResponseWriter, r *http.Request) {
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

// DeletePost godoc
// @Summary      Delete a post
// @Description  Author only
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /posts/{id} [delete]
func (h *handler) DeletePost(w http.ResponseWriter, r *http.Request) {
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

// PinPost godoc
// @Summary      Pin a classroom post
// @Description  Teacher only; notifies approved students
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        classroomId path int true "Classroom ID"
// @Param        id path int true "Post ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /classrooms/{classroomId}/posts/{id}/pin [post]
func (h *handler) PinPost(w http.ResponseWriter, r *http.Request) {
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

// UnpinPost godoc
// @Summary      Unpin a classroom post
// @Description  Teacher only
// @Tags         Posts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        classroomId path int true "Classroom ID"
// @Param        id path int true "Post ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /classrooms/{classroomId}/posts/{id}/pin [delete]
func (h *handler) UnpinPost(w http.ResponseWriter, r *http.Request) {
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
