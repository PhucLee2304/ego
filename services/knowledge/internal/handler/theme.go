package handler

import (
	"net/http"

	"ego/platform/httpx"
	_ "ego/services/knowledge/internal/dto"
)

// GetThemes godoc
// @Summary      Get themes
// @Description  Vocabulary themes ordered for display
// @Tags         Themes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {array}   dto.Theme
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /themes [get]
func (h *handler) GetThemes(w http.ResponseWriter, r *http.Request) {
	themes, httpErr := h.service.GetThemes(r.Context())
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, themes)
}

// GetWordsByTheme godoc
// @Summary      Get theme words
// @Description  Words of a theme in seed order; id is 0 when the word is not in the dictionary
// @Tags         Themes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id        path      int     true   "Theme ID"
// @Param        page      query     int     false  "Page number" default(1)
// @Param        pageSize  query     int     false  "Page size"   default(10)
// @Success      200       {object}  dto.GetThemeWordsResponse
// @Failure      400       {object}  httpx.HTTPError
// @Failure      401       {object}  httpx.HTTPError
// @Failure      404       {object}  httpx.HTTPError
// @Failure      500       {object}  httpx.HTTPError
// @Router       /themes/{id} [get]
func (h *handler) GetWordsByTheme(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid theme ID", err))
		return
	}

	var query httpx.PaginationQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	response, httpErr := h.service.GetWordsByTheme(r.Context(), id, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, response)
}
