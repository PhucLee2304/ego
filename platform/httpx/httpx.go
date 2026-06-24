package httpx

import (
	"ego/platform/logger"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/schema"
)

var (
	decoder      = schema.NewDecoder()
	queryEncoder = schema.NewEncoder()
	validate     = func() *validator.Validate {
		v := validator.New()
		_ = v.RegisterValidation("notblank", func(fl validator.FieldLevel) bool {
			return strings.TrimSpace(fl.Field().String()) != ""
		})
		return v
	}()
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type PaginatedResponse[T any] struct {
	Data       []T `json:"data"`
	Page       int `json:"page"`
	PageCounts int `json:"pageCounts"`
}

type PaginationQuery struct {
	Page     int32 `schema:"page"`
	PageSize int32 `schema:"pageSize"`
}

func (q *PaginationQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 10
	} else if q.PageSize > 100 {
		q.PageSize = 100
	}
}

func (q *PaginationQuery) Limit() int32 {
	return q.PageSize
}

func (q *PaginationQuery) Offset() int32 {
	return (q.Page - 1) * q.PageSize
}

func Error(w http.ResponseWriter, code int, message string) {
	if code >= http.StatusInternalServerError {
		logger.Log.Error().Int("code", code).Msg("Internal server error")
	}

	JSON(w, code, ErrorResponse{Error: message})
}

func DecodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return validate.Struct(v)
}

func DecodeQuery(r *http.Request, v any) error {
	if err := decoder.Decode(v, r.URL.Query()); err != nil {
		return err
	}
	return validate.Struct(v)
}

func EncodeQuery(v any) (url.Values, error) {
	values := url.Values{}
	err := queryEncoder.Encode(v, values)
	return values, err
}

func JSON(w http.ResponseWriter, code int, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.Log.Error().Err(err).Msg("[INTERNAL_SERVER_ERROR] Error marshalling response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(data)
}
