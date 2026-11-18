package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// CreateAttempt godoc
// @Summary      Tạo lượt làm bài
// @Description  Tạo lượt làm bài mới cho một đề ở chế độ luyện tập (PRACTICE), thi (TEST) hoặc bài tập (ASSIGNMENT), có thể giới hạn theo section, part và thời gian. Với bài tập lớp, gửi contextType = CLASSROOM_ASSIGNMENT, contextId là ID assignment, và integrityConsent nếu bài có giám sát.
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path      int                       true  "Exam ID"
// @Param        body  body      dto.CreateExamAttemptRequest  true  "Attempt payload"
// @Success      201  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /exams/{id}/attempts [post]
func (h *handler) CreateAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid exam ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var body dto.CreateExamAttemptRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	body.Normalize()

	if err := body.Validate(); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, err.Error(), err))
		return
	}

	resp, httpErr := h.service.CreateAttempt(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusCreated, resp)
}

// GetAttempts godoc
// @Summary      Lấy danh sách lượt làm bài theo trạng thái
// @Description  Lấy các lượt làm bài của người dùng hiện tại theo trạng thái bắt buộc, có phân trang và lọc theo chế độ, loại đề, tên đề. Không kèm câu hỏi. Client dùng status=SUBMITTED cho màn lịch sử và thống kê, status=ACTIVE để tìm lượt đang làm dở; nội dung xem lại của từng lượt lấy qua /attempts/{id}/history.
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        page      query     int     false  "Page number" default(1)
// @Param        pageSize  query     int     false  "Page size" default(10)
// @Param        status    query     string  true   "Attempt status" Enums(ACTIVE, SUBMITTED, CANCELLED)
// @Param        excludeMode  query  string  false  "Exclude attempts of this mode" Enums(PRACTICE, TEST, ASSIGNMENT)
// @Param        examType     query  string  false  "Comma-separated exam types, e.g. TOEIC,THPT"
// @Param        search       query  string  false  "Search by exam title"
// @Success      200  {object}  httpx.PaginatedResponse[dto.AttemptResponse]
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts [get]
func (h *handler) GetAttempts(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var query dto.GetAttemptsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid query parameters", err))
		return
	}
	query.Normalize()

	resp, total, httpErr := h.service.GetAttempts(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, total))
}

// GetAttemptQuestions godoc
// @Summary      Lấy câu hỏi của lượt làm bài
// @Description  Lấy câu hỏi theo phạm vi đã chọn khi tạo lượt làm bài (section, part), kèm các đáp án đã lưu.
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "Attempt ID"
// @Success      200  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/questions [get]
func (h *handler) GetAttemptQuestions(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	resp, httpErr := h.service.GetAttemptQuestions(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// CancelAttempt godoc
// @Summary      Hủy lượt làm bài
// @Description  Hủy lượt làm bài đang làm, không chấm điểm, và lưu bản chụp lịch sử để xem lại.
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Attempt ID"
// @Success      200  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/cancel [post]
func (h *handler) CancelAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	resp, httpErr := h.service.CancelAttempt(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// GetAttemptHistory godoc
// @Summary      Lấy lịch sử lượt làm bài
// @Description  Lấy bản chụp để xem lại của lượt làm bài đã nộp hoặc đã hủy; bản chụp không đổi khi đề bị sửa sau đó.
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "Attempt ID"
// @Success      200  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/history [get]
func (h *handler) GetAttemptHistory(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	resp, httpErr := h.service.GetAttemptHistory(r.Context(), userID, id)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// SubmitAttempt godoc
// @Summary      Nộp bài
// @Description  Lưu đáp án cuối cùng, chấm điểm và lưu bản chụp lịch sử của lượt làm bài.
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                       true  "Attempt ID"
// @Param        body  body  dto.SubmitAttemptRequest  true  "Final answers"
// @Success      200  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.HTTPError
// @Failure      401  {object}  httpx.HTTPError
// @Failure      404  {object}  httpx.HTTPError
// @Failure      409  {object}  httpx.HTTPError
// @Failure      500  {object}  httpx.HTTPError
// @Router       /attempts/{id}/submit [post]
func (h *handler) SubmitAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid attempt ID", err))
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, httpx.NewError(http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found", nil))
		return
	}

	var body dto.SubmitAttemptRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, "[ERROR] Invalid request body", err))
		return
	}
	if err := body.Validate(); err != nil {
		httpx.Error(w, httpx.NewError(http.StatusBadRequest, err.Error(), err))
		return
	}

	resp, httpErr := h.service.SubmitAttempt(r.Context(), userID, id, body)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
