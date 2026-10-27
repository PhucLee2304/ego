package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

// GetList godoc
// @Summary      Get exams list
// @Description  Get the list of all exams
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Param        page      query     int  false  "Page number" default(1)
// @Param        pageSize  query     int  false  "Page size"   default(10)
// @Param        type      query     string  false  "Exam type" Enums(THPT, TOEIC) default(THPT)
// @Success      200  {object}  httpx.PaginatedResponse[dto.GetExamsResponse]
// @Failure      400  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /exams [get]
func (h *handler) GetList(w http.ResponseWriter, r *http.Request) {
	var query dto.GetExamsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	exams, pageCounts, httpErr := h.service.GetList(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(exams, query.PaginationQuery, pageCounts))
}

// GetByID godoc
// @Summary      Get exam by ID
// @Description  Get exam detail by ID
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Param        id   path      int  true  "Exam ID"
// @Success      200  {object}  dto.GetExamResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /exams/{id} [get]
func (h *handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid exam ID", err))
		return
	}

	resp, httpErr := h.service.GetByID(r.Context(), id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
