package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/services/learning/internal/dto"
)

// GetList godoc
// @Summary      Lấy danh sách đề thi
// @Description  Trả danh sách đề công khai có phân trang, lọc theo loại đề, năm và từ khóa trong tiêu đề. Không gồm đề riêng của bài tập lớp.
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int  false  "Page number" default(1)
// @Param        pageSize  query     int  false  "Page size"   default(10)
// @Param        type      query     string  false  "Exam type" Enums(THPT, TOEIC) default(THPT)
// @Param        search    query     string  false  "Search by title"
// @Param        year      query     int     false  "Exam year"
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

	exams, total, httpErr := h.service.GetList(r.Context(), query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(exams, query.PaginationQuery, total))
}

// GetYears godoc
// @Summary      Lấy danh sách năm của đề thi
// @Description  Trả các năm (không trùng) có đề công khai thuộc loại đề đã chọn, năm mới nhất trước. Dùng cho bộ lọc theo năm.
// @Tags         Exams
// @Produce      json
// @Security     BearerAuth
// @Param        type  query     string  false  "Exam type" Enums(THPT, TOEIC) default(THPT)
// @Success      200   {array}   int
// @Failure      400   {object}  httpx.HTTPError
// @Failure      500   {object}  httpx.HTTPError
// @Router       /exams/years [get]
func (h *handler) GetYears(w http.ResponseWriter, r *http.Request) {
	var query dto.GetExamsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	years, httpErr := h.service.GetExamYears(r.Context(), query.Type)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, years)
}

// GetByID godoc
// @Summary      Lấy chi tiết đề thi
// @Description  Trả thông tin tổng quan của đề công khai: thời gian làm bài, số câu, các phần thi và số câu mỗi part. Không gồm nội dung câu hỏi; câu hỏi chỉ trả khi bắt đầu làm bài.
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Security     BearerAuth
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
