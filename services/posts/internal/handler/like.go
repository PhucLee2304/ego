package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
)

// LikePost godoc
// @Summary      Like a post
// @Description  Idempotent; notifies the post author on the first like
// @Tags         Likes
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
// @Router       /posts/{id}/like [post]
func (h *handler) LikePost(w http.ResponseWriter, r *http.Request) {
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

// UnlikePost godoc
// @Summary      Unlike a post
// @Tags         Likes
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
// @Router       /posts/{id}/like [delete]
func (h *handler) UnlikePost(w http.ResponseWriter, r *http.Request) {
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

// LikePostComment godoc
// @Summary      Like a comment
// @Description  Idempotent; notifies the comment author on the first like
// @Tags         Likes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Comment ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /comments/{id}/like [post]
func (h *handler) LikePostComment(w http.ResponseWriter, r *http.Request) {
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

// UnlikePostComment godoc
// @Summary      Unlike a comment
// @Tags         Likes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Comment ID"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /comments/{id}/like [delete]
func (h *handler) UnlikePostComment(w http.ResponseWriter, r *http.Request) {
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
