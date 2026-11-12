package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/knowledge/internal/dto"
)

// CreateDeck godoc
// @Summary      Create flashcard deck
// @Description  Create a new named collection of flashcards
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body      dto.CreateDeckBody  true  "Deck"
// @Success      201      {object}  dto.DeckResponse
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcard-decks [post]
func (h *handler) CreateDeck(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	var body dto.CreateDeckBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	deck, httpErr := h.service.CreateDeck(r.Context(), userID, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusCreated, deck)
}

// GetDecks godoc
// @Summary      List flashcard decks
// @Description  Get all flashcard decks belonging to the current user
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   dto.DeckResponse
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcard-decks [get]
func (h *handler) GetDecks(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	decks, httpErr := h.service.GetDecks(r.Context(), userID)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, decks)
}

// GetDeckByID godoc
// @Summary      Get flashcard deck
// @Description  Get a single flashcard deck by ID
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Deck ID"
// @Success      200  {object}  dto.DeckResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcard-decks/{id} [get]
func (h *handler) GetDeckByID(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid deck ID", err))
		return
	}

	deck, httpErr := h.service.GetDeckByID(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, deck)
}

// UpdateDeck godoc
// @Summary      Update flashcard deck
// @Description  Update name, description, or color of a deck
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path      int                  true  "Deck ID"
// @Param        request  body      dto.UpdateDeckBody   true  "Update fields"
// @Success      200      {object}  dto.DeckResponse
// @Failure      400      {object}  httpx.HTTPError
// @Failure      401      {object}  httpx.HTTPError
// @Failure      404      {object}  httpx.HTTPError
// @Failure      500      {object}  httpx.HTTPError
// @Router       /flashcard-decks/{id} [put]
func (h *handler) UpdateDeck(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid deck ID", err))
		return
	}

	var body dto.UpdateDeckBody
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}

	deck, httpErr := h.service.UpdateDeck(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, deck)
}

// DeleteDeck godoc
// @Summary      Delete flashcard deck
// @Description  Delete a flashcard deck and all its cards
// @Tags         Flashcards
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path  int  true  "Deck ID"
// @Success      204  "No Content"
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /flashcard-decks/{id} [delete]
func (h *handler) DeleteDeck(w http.ResponseWriter, r *http.Request) {
	userID, _ := jwt.GetUserID(r.Context())

	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid deck ID", err))
		return
	}

	if httpErr := h.service.DeleteDeck(r.Context(), userID, id); httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
