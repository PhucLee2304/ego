package valuex

func MapPtr[T any, U any](value *T, fn func(T) U) *U {
	if value == nil {
		return nil
	}

	mapped := fn(*value)
	return &mapped
}
