package dummy

import (
	"context"
)

type StreamEncoder struct{}

func NewStreamEncoder() *StreamEncoder {
	return new(StreamEncoder)
}

func (e *StreamEncoder) Instance() any {
  return nil
}

func (e *StreamEncoder) Elem() any {
  return nil
}

func (e *StreamEncoder) EnableValidate() {}

func (e *StreamEncoder) Context() []byte {
	return nil
}

func (e *StreamEncoder) Read(ctx context.Context, ch <-chan string) <-chan any {
	parsedChan := make(chan any)
	go func() {
		defer close(parsedChan)
		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-ch:
				if !ok {
					// Stream closed
					return
				}
				parsedChan <- text
			}
		}
	}()
	return parsedChan
}
