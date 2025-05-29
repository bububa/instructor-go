package instructor

import "context"

type Encoder interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
	Context() []byte
  Instance() any
  Elem() any
}

type Validator interface {
	Validate(any) error
}

type StreamEncoder interface {
	Read(context.Context, <-chan string) <-chan any
	Context() []byte
	EnableValidate()
  Instance() any
  Elem() any
}

type Faker interface {
	Fake() any
}
