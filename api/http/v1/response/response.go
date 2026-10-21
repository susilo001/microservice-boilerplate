package response

import (
	"fmt"
	"time"
)

type APIResponse[T any] struct {
	CorrelationID string    `json:"correlationid"`
	Error         string    `json:"error"`
	Tin           time.Time `json:"tin"`
	Tout          time.Time `json:"tout"`
	Data          T         `json:"data"`
}

type Empty struct{}

type ListData[T any] struct {
	List       []T       `json:"list"`
	TotalItems int64     `json:"total_items"`
	TotalPages int64     `json:"total_pages"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	Start      time.Time `json:"start"`
	Finish     time.Time `json:"finish"`
	Duration   string    `json:"duration"`
}

func NewSuccess[T any](correlationID string, tin, tout time.Time, data T) APIResponse[T] {
	return APIResponse[T]{
		CorrelationID: correlationID,
		Tin:           tin,
		Tout:          tout,
		Data:          data,
	}
}

func NewError(correlationID string, tin, tout time.Time, errMsg string) APIResponse[Empty] {
	return APIResponse[Empty]{
		CorrelationID: correlationID,
		Error:         errMsg,
		Tin:           tin,
		Tout:          tout,
		Data:          Empty{},
	}
}

func NewList[T any](correlationID string, tin, tout time.Time, list []T, page, pageSize int, totalItems int64, start, finish time.Time) APIResponse[ListData[T]] {
	if list == nil {
		list = []T{}
	}
	totalPages := int64(0)
	if pageSize > 0 && totalItems > 0 {
		totalPages = (totalItems + int64(pageSize) - 1) / int64(pageSize)
	}
	return APIResponse[ListData[T]]{
		CorrelationID: correlationID,
		Tin:           tin,
		Tout:          tout,
		Data: ListData[T]{
			List:       list,
			TotalItems: totalItems,
			TotalPages: totalPages,
			Page:       page,
			PageSize:   pageSize,
			Start:      start,
			Finish:     finish,
			Duration:   fmt.Sprintf("%.2f ms", float64(finish.Sub(start).Nanoseconds())/1e6),
		},
	}
}

// ContextTime reads a time.Time stored in a Gin context (e.g. by RequestTimer).
// Falls back to time.Now() if the key is absent.
func ContextTime(c interface{ Get(string) (any, bool) }, key string) time.Time {
	if v, ok := c.Get(key); ok {
		if t, ok := v.(time.Time); ok {
			return t
		}
	}
	return time.Now()
}
