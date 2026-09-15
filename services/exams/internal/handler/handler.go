package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/service"
	"errors"
	"net/http"
	"strings"

	"gorm.io/gorm"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, authMw *jwt.AuthMiddleware)
	GetList(w http.ResponseWriter, r *http.Request)
	GetByID(w http.ResponseWriter, r *http.Request)
	CreateAttempt(w http.ResponseWriter, r *http.Request)
	GetAttempts(w http.ResponseWriter, r *http.Request)
	GetAttemptQuestions(w http.ResponseWriter, r *http.Request)
	SubmitAttempt(w http.ResponseWriter, r *http.Request)
	CancelAttempt(w http.ResponseWriter, r *http.Request)
	GetAttemptHistory(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	service service.Service
}

func New(svc service.Service) Handler {
	return &handler{service: svc}
}

func (h *handler) RegisterRoutes(mux *http.ServeMux, mw *jwt.AuthMiddleware) {
	mux.HandleFunc("GET /exams", mw.Handle(h.GetList))
	mux.HandleFunc("GET /exams/{id}", mw.Handle(h.GetByID))
	mux.HandleFunc("POST /exams/{id}/attempts", mw.Handle(h.CreateAttempt))
	mux.HandleFunc("GET /attempts", mw.Handle(h.GetAttempts))
	mux.HandleFunc("GET /attempts/{id}/questions", mw.Handle(h.GetAttemptQuestions))
	mux.HandleFunc("POST /attempts/{id}/submit", mw.Handle(h.SubmitAttempt))
	mux.HandleFunc("POST /attempts/{id}/cancel", mw.Handle(h.CancelAttempt))
	mux.HandleFunc("GET /attempts/{id}/history", mw.Handle(h.GetAttemptHistory))
}

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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /exams [get]
func (h *handler) GetList(w http.ResponseWriter, r *http.Request) {
	var query dto.GetExamsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()

	exams, pageCounts, err := h.service.GetList(r.Context(), query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /exams/{id} [get]
func (h *handler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid exam ID")
		return
	}

	resp, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, http.StatusNotFound, "[ERROR] Exam not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// CreateAttempt godoc
// @Summary      Create exam attempt
// @Description  Create a new practice or test attempt for an exam
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "Exam ID"
// @Param        body  body      dto.CreateExamAttemptRequest  true  "Attempt payload"
// @Success      201  {object}  dto.AttemptResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      409  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /exams/{id}/attempts [post]
func (h *handler) CreateAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid exam ID")
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var body dto.CreateExamAttemptRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	body.Normalize()

	if err := body.Validate(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.CreateAttempt(r.Context(), userID, id, body)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Exam not found")
			return
		case strings.HasPrefix(err.Error(), "[CONFLICT]"):
			httpx.Error(w, http.StatusConflict, err.Error())
			return
		case strings.HasPrefix(err.Error(), "[ERROR]"):
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /attempts [get]
func (h *handler) GetAttempts(w http.ResponseWriter, r *http.Request) {
	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var query dto.GetAttemptsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()
	if err := query.Validate(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, pageCounts, err := h.service.GetAttempts(r.Context(), userID, query)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /attempts/{id}/questions [get]
func (h *handler) GetAttemptQuestions(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid attempt ID")
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	resp, err := h.service.GetAttemptQuestions(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, http.StatusNotFound, "[ERROR] Attempt not found")
			return
		}
		if strings.HasPrefix(err.Error(), "[ERROR]") {
			httpx.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      409  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /attempts/{id}/submit [post]
func (h *handler) SubmitAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid attempt ID")
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	var body dto.SubmitAttemptRequest
	if err := httpx.DecodeJSON(r, &body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid request body")
		return
	}
	if err := body.Validate(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.SubmitAttempt(r.Context(), userID, id, body)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Attempt not found")
		case errors.Is(err, service.ErrAttemptCancelled), errors.Is(err, service.ErrAttemptAlreadySubmitted):
			httpx.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrQuestionNotBelongToAttempt), errors.Is(err, service.ErrOptionNotBelongToQuestion):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      409  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /attempts/{id}/cancel [post]
func (h *handler) CancelAttempt(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid attempt ID")
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	resp, err := h.service.CancelAttempt(r.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.Error(w, http.StatusNotFound, "[ERROR] Attempt not found")
		case errors.Is(err, service.ErrAttemptAlreadySubmitted), errors.Is(err, service.ErrAttemptCancelled):
			httpx.Error(w, http.StatusConflict, err.Error())
		case errors.Is(err, service.ErrAttemptNotActive):
			httpx.Error(w, http.StatusBadRequest, err.Error())
		default:
			httpx.Error(w, http.StatusInternalServerError, err.Error())
		}
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
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /attempts/{id}/history [get]
func (h *handler) GetAttemptHistory(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.ParsePathID(r, "id")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid attempt ID")
		return
	}

	userID, ok := jwt.GetUserID(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "[UNAUTHORIZED] User ID not found")
		return
	}

	resp, err := h.service.GetAttemptHistory(r.Context(), userID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, http.StatusNotFound, "[ERROR] Attempt history not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.JSON(w, http.StatusOK, resp)
}
