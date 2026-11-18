package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/ai/internal/dto"
)

// Chat godoc
// @Summary      AI Tutor chat
// @Description  Gửi câu hỏi, nhận câu trả lời từ AI Tutor (Gemini)
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      dto.ChatRequest   true  "Chat request"
// @Success      200   {object}  dto.ChatResponse
// @Failure      400   {object}  httpx.HTTPError
// @Failure      401   {object}  httpx.HTTPError
// @Failure      500   {object}  httpx.HTTPError
// @Router       /chat [post]
func (h *handler) Chat(w http.ResponseWriter, r *http.Request) {
	_, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	var req dto.ChatRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.Chat(r.Context(), req)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
