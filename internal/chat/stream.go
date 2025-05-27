package chat

import (
	"context"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

const WRAPPER_END = `"items": [`

func SchemaStreamHandler[T any, RESP any](i instructor.SchemaStreamInstructor[T, RESP], ctx context.Context, request *T, responseType any, resp *RESP) (<-chan any, <-chan instructor.StreamData, error) {
	var (
		enc = i.StreamEncoder()
		err error
	)
	if enc == nil {
		if enc, err = encoding.PredefinedStreamEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
			return nil, nil, err
		}
		i.SetStreamEncoder(enc)
	}
	ch, err := i.SchemaStreamHandler(ctx, request, resp)
	if err != nil {
		return nil, nil, err
	}
	if i.Validate() {
		enc.EnableValidate()
	}
	contentCh := make(chan string)
	outputCh := make(chan instructor.StreamData)
	go func() {
		defer close(contentCh)
		defer close(outputCh)
		for item := range ch {
			outputCh <- item
			if item.Type == instructor.ContentStream {
				contentCh <- item.Content
			}
		}
	}()
	parsedChan := enc.Read(ctx, contentCh)

	return parsedChan, outputCh, nil
}
