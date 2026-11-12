package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/identity/internal/dto"
)

// GetAdminStats godoc
// @Summary      Get user stats
// @Description  Admin statistics for users: current totals, sign-ups (per day and weekday × hour) and locked accounts. Range is inclusive dates in Asia/Ho_Chi_Minh; omit both for all time.
// @Tags         Stats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        from query string false "First day (YYYY-MM-DD)"
// @Param        to query string false "Last day (YYYY-MM-DD), required with from"
// @Success      200 {object} dto.AdminStatsResponse
// @Failure      400 {object} httpx.HTTPError
// @Failure      401 {object} httpx.HTTPError
// @Failure      403 {object} httpx.HTTPError
// @Failure      500 {object} httpx.HTTPError
// @Router       /admin/stats [get]
func (h *handler) GetAdminStats(w http.ResponseWriter, r *http.Request) { // **Reviewed**
	var query dto.AdminStatsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	resp, httpErr := h.service.GetAdminStats(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}
