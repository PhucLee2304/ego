package handler

import (
	"net/http"

	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/learning/internal/dto"
)

// CreateAttempt godoc
// @Summary      Create exam attempt
// @Description  Create a new practice or test attempt for an exam
// @Tags         Exams
// @Accept       json
// @Produce      json
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
// @Summary      Get attempts by status
// @Description  Get current user's attempts by required status
// @Tags         Attempts
// @Accept       json
// @Produce      json
// @Param        page      query     int     false  "Page number" default(1)
// @Param        pageSize  query     int     false  "Page size" default(10)
// @Param        status    query     string  true   "Attempt status" Enums(ACTIVE, SUBMITTED, CANCELLED)
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

	resp, pageCounts, httpErr := h.service.GetAttempts(r.Context(), userID, query)
	if httpErr != nil {
		httpx.Error(w, httpErr)
		return
	}

	httpx.JSON(w, http.StatusOK, httpx.ToPaginatedResponse(resp, query.PaginationQuery, pageCounts))
}

// GetAttemptQuestions godoc
// @Summary      Get attempt questions
// @Description  Get questions by attempt context with saved answers
// @Tags         Attempts
// @Accept       json
// @Produce      json
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
// @Summary      Cancel attempt
// @Description  Cancel an active attempt without grading it and create a history snapshot
// @Tags         Attempts
// @Accept       json
// @Produce      json
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
// @Summary      Get attempt history
// @Description  Get the immutable review snapshot of a submitted or cancelled attempt
// @Tags         Attempts
// @Accept       json
// @Produce      json
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
// @Summary      Submit attempt
// @Description  Save the final answers, grade the attempt and create its history snapshot
// @Tags         Attempts
// @Accept       json
// @Produce      json
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
