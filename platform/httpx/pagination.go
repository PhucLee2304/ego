package httpx

type PaginatedResponse[T any] struct {
	Data       []T   `json:"data"`
	Page       int   `json:"page"`
	PageCounts int   `json:"pageCounts"`
	Total      int64 `json:"total,omitempty"`
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

func ToPaginatedResponse[T any](data []T, query PaginationQuery, total int64) PaginatedResponse[T] {
	return PaginatedResponse[T]{
		Data:       data,
		Page:       int(query.Page),
		PageCounts: ToPageCounts(total, query.PageSize),
		Total:      total,
	}
}
