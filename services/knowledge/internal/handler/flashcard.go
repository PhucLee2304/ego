package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/knowledge/internal/dto"
)

// GetFlashcards godoc
// @Summary      Get my flashcards
// @Description  Flashcards saved by the current user, newest first
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int  false  "Page number" default(1)
// @Param        pageSize  query     int  false  "Page size"   default(10)
// @Success      200       {object}  httpx.PaginatedResponse[dto.Flashcard]
// @Failure      400       {object}  httpx.HTTPError
// @Failure      401       {object}  httpx.HTTPError
// @Failure      500       {object}  httpx.HTTPError
// @Router       /flashcards [get]
func (h *handler) GetFlashcards(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var query httpx.PaginationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	flashcards, total, httpErr := h.service.GetFlashcards(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(flashcards, query, total))
}

// CreateFlashcard godoc
// @Summary      Create flashcard
// @Description  Save a dictionary word to the current user's flashcards with an optional note
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      dto.CreateFlashcardBody  true  "Flashcard"
// @Success      201      {object}  dto.Flashcard
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcards [post]
func (h *handler) CreateFlashcard(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var body dto.CreateFlashcardBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	flashcard, httpErr := h.service.CreateFlashcard(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusCreated, flashcard)
}

// DeleteFlashcard godoc
// @Summary      Delete flashcard
// @Description  Remove one of the current user's flashcards
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Flashcard ID"
// @Success      204  "No Content"
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcards/{id} [delete]
func (h *handler) DeleteFlashcard(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid flashcard ID", err))
		return
	}

	if httpErr := h.service.DeleteFlashcard(r.Context(), userID, id); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
