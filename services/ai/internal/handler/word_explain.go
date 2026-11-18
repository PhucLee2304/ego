package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/ai/internal/dto"
)

// ExplainWord godoc
// @Summary      AI giải thích từ vựng (RAG)
// @Description  Nhận dữ liệu từ điển từ Flutter, xây dựng RAG context, trả về giải thích chi tiết từ AI
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      dto.WordExplainRequest   true  "Word explain request"
// @Success      200   {object}  dto.WordExplainResponse
// @Failure      400   {object}  httpx.HTTPError
// @Failure      401   {object}  httpx.HTTPError
// @Failure      500   {object}  httpx.HTTPError
// @Router       /words/explain [post]
func (h *handler) ExplainWord(w http.ResponseWriter, r *http.Request) {
	// Kiểm tra xác thực
	_, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	// Decode request body sử dụng helper chuẩn của platform
	var req dto.WordExplainRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	// Gọi service để xây dựng RAG context và lấy giải thích AI
	resp, httpErr := h.service.ExplainWord(r.Context(), req)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
