package chat

import (
	"context"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

const WRAPPER_END = `"items": [`

func SchemaStreamHandler[T any, RESP any](i instructor.SchemaStreamInstructor[T, RESP], ctx context.Context, request *T, responseType any, resp *RESP) (<-chan any, error) {
	var (
		enc = i.StreamEncoder()
		err error
	)
	if enc == nil {
		if enc, err = encoding.PredefinedStreamEncoder(i.Mode(), responseType); err != nil {
			return nil, err
		}
		i.SetStreamEncoder(enc)
	}
	ch, err := i.SchemaStreamHandler(ctx, request, resp)
	if err != nil {
		return nil, err
	}
	if i.Validate() {
		enc.EnableValidate()
	}

	parsedChan := enc.Read(ctx, ch)

	return parsedChan, nil
}
