package httpx

import (
	"ego/platform/validatorx"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gorilla/schema"
)

var (
	decoder      = schema.NewDecoder()
	queryEncoder = schema.NewEncoder()
	validate     = validatorx.New()
)

type PaginatedResponse[T any] struct {
	Data       []T `json:"data"`
	Page       int `json:"page"`
	PageCounts int `json:"pageCounts"`
}

type PaginationQuery struct {
	Page     int32 `schema:"page" default:"1"`
	PageSize int32 `schema:"pageSize" default:"10"`
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

func ToPageCounts(total int64, pageSize int32) int {
	pageCounts := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pageCounts == 0 {
		return 1
	}
	return pageCounts
}

func ToPaginatedResponse[T any](data []T, query PaginationQuery, pageCounts int) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Data:       data,
		Page:       int(query.Page),
		PageCounts: pageCounts,
	}
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

func ParsePathID(r *http.Request, name string) (uint, error) {
	id, err := strconv.ParseUint(r.PathValue(name), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func EncodeQuery(v any) (url.Values, error) {
	values := url.Values{}
	err := queryEncoder.Encode(v, values)
	return values, err
}
