package instructor

import "context"

type Encoder interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
	Context() []byte
}

type Validator interface {
	Validate(any) error
}

type StreamEncoder interface {
	Read(context.Context, <-chan string) <-chan any
	Context() []byte
	EnableValidate()
}
