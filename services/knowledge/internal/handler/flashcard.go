package handler

import (
	"net/http"
	"strconv"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/knowledge/internal/dto"
)

// SaveWordToDecks godoc
// @Summary      Save word to decks
// @Description  Save a dictionary word into one or more flashcard decks (optionally creating a new deck)
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  dto.SaveWordToDecksBody  true  "Save request"
// @Success      200      "OK"
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcards/save-word [post]
func (h *handler) SaveWordToDecks(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var body dto.SaveWordToDecksBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	if httpErr := h.service.SaveWordToDecks(r.Context(), userID, body); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetWordSaveStatus godoc
// @Summary      Get word save status
// @Description  Check whether a word is saved in any deck and which deck IDs it belongs to
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        wordId  query     int  true  "Word ID"
// @Success      200     {object}  dto.WordSaveStatusResponse
// @Failure      400     {object}  httpx.HTTPError
// @Failure      401     {object}  httpx.HTTPError
// @Failure      500     {object}  httpx.HTTPError
// @Router       /flashcards/status [get]
func (h *handler) GetWordSaveStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	q := r.URL.Query().Get("wordId")
	wordID, err := strconv.ParseUint(q, 10, 64)
	if err != nil || wordID == 0 {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid or missing wordId query parameter", err))
		return
	}

	status, httpErr := h.service.GetWordSaveStatus(r.Context(), userID, uint(wordID))
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, status)
}

// GetCardsByDeck godoc
// @Summary      List cards in a deck
// @Description  Get paginated flashcards belonging to a specific deck
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      int  true   "Deck ID"
// @Param        page      query     int  false  "Page number"  default(1)
// @Param        pageSize  query     int  false  "Page size"    default(10)
// @Success      200       {object}  httpx.PaginatedResponse[dto.Flashcard]
// @Failure      400       {object}  httpx.HTTPError
// @Failure      401       {object}  httpx.HTTPError
// @Failure      404       {object}  httpx.HTTPError
// @Failure      500       {object}  httpx.HTTPError
// @Router       /flashcard-decks/{id}/cards [get]
func (h *handler) GetCardsByDeck(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	deckID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid deck ID", err))
		return
	}

	var query httpx.PaginationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	cards, total, httpErr := h.service.GetCardsByDeck(r.Context(), userID, deckID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(cards, query, total))
}

// UpdateFlashcardNote godoc
// @Summary      Update flashcard note
// @Description  Update or clear the personal note attached to a flashcard
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                         true  "Flashcard ID"
// @Param        request  body      dto.UpdateFlashcardNoteBody true  "Note"
// @Success      200      {object}  dto.Flashcard
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcards/{id}/note [put]
func (h *handler) UpdateFlashcardNote(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid flashcard ID", err))
		return
	}

	var body dto.UpdateFlashcardNoteBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	card, httpErr := h.service.UpdateFlashcardNote(r.Context(), userID, id, body.Note)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, card)
}

// DeleteFlashcard godoc
// @Summary      Delete flashcard
// @Description  Remove a flashcard from its deck
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  int  true  "Flashcard ID"
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

// GetStudyCards godoc
// @Summary      Get study cards for a deck
// @Description  Get flashcards due for review in a specific deck (SRS schedule)
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Deck ID"
// @Success      200  {array}   dto.Flashcard
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcard-decks/{id}/study [get]
func (h *handler) GetStudyCards(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	deckID, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid deck ID", err))
		return
	}

	cards, httpErr := h.service.GetStudyCards(r.Context(), userID, deckID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, cards)
}

// GetAllDueCards godoc
// @Summary      Get all due cards
// @Description  Get all flashcards due for review across every deck of the current user
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   dto.Flashcard
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcards/study/due-all [get]
func (h *handler) GetAllDueCards(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	cards, httpErr := h.service.GetAllDueCards(r.Context(), userID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, cards)
}

// ReviewFlashcard godoc
// @Summary      Review a flashcard
// @Description  Submit a review rating for a flashcard; updates SRS scheduling fields
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                  true  "Flashcard ID"
// @Param        request  body      dto.ReviewCardBody   true  "Review result"
// @Success      200      {object}  dto.ReviewCardResponse
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcards/{id}/review [post]
func (h *handler) ReviewFlashcard(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid flashcard ID", err))
		return
	}

	var body dto.ReviewCardBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	result, httpErr := h.service.ReviewFlashcard(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, result)
}

// GetFlashcardUserStats godoc
// @Summary      Get flashcard stats
// @Description  Get overall flashcard statistics for the current user (total, mastered, due today, streak)
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  dto.FlashcardUserStats
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcards/stats [get]
func (h *handler) GetFlashcardUserStats(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	stats, httpErr := h.service.GetFlashcardUserStats(r.Context(), userID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, stats)
}
