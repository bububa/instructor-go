package internal

func ToPtr[T any](val T) *T {
	return &val
}

func Prepend[T any](to []T, from T) []T {
	return append([]T{from}, to...)
}

func IsAllSameByte(s []byte, target byte) bool {
    if len(s) == 0 {
        return false // 空字符串返回 false [[3]][[8]]
    }
    for _, v := range s {
        if v != target {
            return false
        }
    }
    return true
}
