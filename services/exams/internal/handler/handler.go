package handler

import (
	"ego/platform/httpx"
	"ego/platform/jwt"
	"ego/services/exams/internal/dto"
	"ego/services/exams/internal/service"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

type Handler interface {
	RegisterRoutes(mux *http.ServeMux, authMw *jwt.AuthMiddleware)
	GetList(w http.ResponseWriter, r *http.Request)
	GetByID(w http.ResponseWriter, r *http.Request)
	GetQuestions(w http.ResponseWriter, r *http.Request)
	CreateAttempt(w http.ResponseWriter, r *http.Request)
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
	mux.HandleFunc("GET /exams/{id}/questions", mw.Handle(h.GetQuestions))
	mux.HandleFunc("POST /exams/{id}/attempts", mw.Handle(h.CreateAttempt))
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

	httpx.JSON(w, http.StatusOK, httpx.PaginatedResponse[*dto.GetExamsResponse]{
		Data:       exams,
		Page:       int(query.Page),
		PageCounts: pageCounts,
	})
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
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid exam ID")
		return
	}

	resp, err := h.service.GetByID(r.Context(), uint(id))
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

// GetQuestions godoc
// @Summary      Get exam questions
// @Description  Get exam questions by exam ID with optional section or part filter
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Param        id       path      int     true   "Exam ID"
// @Param        section  query     string  false  "Section code" Enums(FULL, LISTENING, READING)
// @Param        part     query     string  false  "TOEIC part" Enums(1, 2, 3, 4, 5, 6, 7)
// @Success      200  {object}  dto.GetExamQuestionsResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /exams/{id}/questions [get]
func (h *handler) GetQuestions(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid exam ID")
		return
	}

	var query dto.GetExamQuestionsQuery
	if err := httpx.DecodeQuery(r, &query); err != nil {
		httpx.Error(w, http.StatusBadRequest, "[ERROR] Invalid query parameters")
		return
	}
	query.Normalize()
	if err := query.Validate(); err != nil {
		httpx.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	resp, err := h.service.GetQuestions(r.Context(), uint(id), query)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, http.StatusNotFound, "[ERROR] Exam not found")
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

// CreateAttempt godoc
// @Summary      Create exam attempt
// @Description  Create a new practice or test attempt for an exam
// @Tags         Exams
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "Exam ID"
// @Param        body  body      dto.CreateExamAttemptRequest  true  "Attempt payload"
// @Success      201  {object}  dto.CreateExamAttemptResponse
// @Failure      400  {object}  httpx.ErrorResponse
// @Failure      401  {object}  httpx.ErrorResponse
// @Failure      404  {object}  httpx.ErrorResponse
// @Failure      409  {object}  httpx.ErrorResponse
// @Failure      500  {object}  httpx.ErrorResponse
// @Router       /exams/{id}/attempts [post]
func (h *handler) CreateAttempt(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
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

	resp, err := h.service.CreateAttempt(r.Context(), userID, uint(id), body)
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
