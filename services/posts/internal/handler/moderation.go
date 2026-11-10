package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/posts/internal/dto"
	"ego/services/posts/internal/model"
)

// ListPostsForModeration godoc
// @Summary      List community posts for moderation
// @Description  Admin only; filter by status, omit for all
// @Tags         Moderation
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        status query string false "PUBLISHED or HIDDEN"
// @Param        page query int false "Page number"
// @Param        pageSize query int false "Page size"
// @Success      200  {object}  dto.PostsResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /admin/posts [get]
func (h *handler) ListPostsForModeration(w http.ResponseWriter, r *http.Request) {
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

// HidePost godoc
// @Summary      Hide a post
// @Description  Admin only; the post stays visible to its author
// @Tags         Moderation
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Param        request body dto.ModerationRequest true "Reason"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /admin/posts/{id}/hide [put]
func (h *handler) HidePost(w http.ResponseWriter, r *http.Request) {
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

// UnhidePost godoc
// @Summary      Unhide a post
// @Description  Admin only
// @Tags         Moderation
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Param        reason query string true "Reason"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /admin/posts/{id}/hide [delete]
func (h *handler) UnhidePost(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var query dto.ModerationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	if httpErr := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationUnhide, query.Reason); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeletePostAsAdmin godoc
// @Summary      Delete a post as admin
// @Description  Admin only; records an audit entry
// @Tags         Moderation
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Param        reason query string true "Reason"
// @Success      204
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /admin/posts/{id} [delete]
func (h *handler) DeletePostAsAdmin(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())
	postID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid ID", err))
		return
	}
	var query dto.ModerationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	if httpErr := h.service.ModeratePost(r.Context(), userID, postID, model.ModerationDelete, query.Reason); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPostModerationAudits godoc
// @Summary      Get moderation audits of a post
// @Description  Admin only; newest first
// @Tags         Moderation
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Post ID"
// @Success      200  {array}   dto.AuditResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      403  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /admin/posts/{id}/audits [get]
func (h *handler) GetPostModerationAudits(w http.ResponseWriter, r *http.Request) {
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
