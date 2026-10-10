package valuex

import "strconv"

func MapPtr[T any, U any](value *T, fn func(T) U) *U {
	if value == nil {
		return nil
	}

	mapped := fn(*value)
	return &mapped
}

func UintStringPtr(value uint) *string {
	result := strconv.FormatUint(uint64(value), 10)
	return &result
}
