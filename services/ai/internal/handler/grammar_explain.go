package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/ai/internal/dto"
)

// ExplainGrammar godoc
// @Summary      Explain a grammar practice answer in Vietnamese
// @Description  Uses the selected answer, question, options, correct answer and existing explanation as context.
// @Tags         AI
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      dto.GrammarExplainRequest   true  "Grammar practice context"
// @Success      200   {object}  dto.GrammarExplainResponse
// @Failure      400   {object}  httpx.HTTPError
// @Failure      401   {object}  httpx.HTTPError
// @Failure      500   {object}  httpx.HTTPError
// @Router       /grammar/explain [post]
func (h *handler) ExplainGrammar(w http.ResponseWriter, r *http.Request) {
	if _, ok := jwt.GetUserID(r.Context()); !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User not authenticated", nil))
		return
	}

	var req dto.GrammarExplainRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	resp, httpErr := h.service.ExplainGrammar(r.Context(), req)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
