package instructor

type Memory[T any] struct {
	list []T
}

func NewMemory[T any](cap int) *Memory[T] {
	return &Memory[T]{
		list: make([]T, 0, cap),
	}
}

func (m *Memory[T]) Set(list []T) {
	m.list = make([]T, len(list))
	copy(m.list, list)
}

func (m *Memory[T]) Add(v ...T) {
	m.list = append(m.list, v...)
}

func (m *Memory[T]) List() []T {
	return m.list
}
