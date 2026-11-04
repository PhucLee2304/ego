package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/knowledge/internal/dto"
)

// GetWords godoc
// @Summary      Get word list
// @Description  Prefix search over dictionary words, ordered alphabetically
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        query     query     string  false  "Word prefix"
// @Param        page      query     int     false  "Page number" default(1)
// @Param        pageSize  query     int     false  "Page size"   default(10)
// @Success      200       {object}  httpx.PaginatedResponse[dto.Word]
// @Failure      400       {object}  httpx.HTTPError
// @Failure      500       {object}  httpx.HTTPError
// @Router       /words [get]
func (h *handler) GetWords(w http.ResponseWriter, r *http.Request) {
	var query dto.GetWordsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	words, total, httpErr := h.service.GetWords(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(words, query.PaginationQuery, total))
}

// GetWordByID godoc
// @Summary      Get word detail
// @Description  Full dictionary entry: phonetics, meanings, definitions, synonyms, antonyms
// @Tags         Dictionary
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Word ID"
// @Success      200  {object}  dto.Word
// @Failure      400  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /words/{id} [get]
func (h *handler) GetWordByID(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid word ID", err))
		return
	}

	word, httpErr := h.service.GetWordByID(r.Context(), id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, word)
}
