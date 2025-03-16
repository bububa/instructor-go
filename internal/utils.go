package internal

func ToPtr[T any](val T) *T {
	return &val
}

func Prepend[T any](to []T, from T) []T {
	return append([]T{from}, to...)
}
