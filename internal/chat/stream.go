package chat

import (
	"context"
	"reflect"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

const WRAPPER_END = `"items": [`

func SchemaStreamHandler[T any, RESP any](i instructor.SchemaStreamInstructor[T, RESP], ctx context.Context, request *T, responseType any, resp *RESP) (<-chan any, <-chan instructor.StreamData, error) {
	var (
		toolCallMode = i.Mode() == instructor.ModeToolCall || i.Mode() == instructor.ModeToolCallStrict
		toolEnc      = i.Encoder()
		enc          = i.StreamEncoder()
		reflectType  = reflect.TypeOf(responseType)
		err          error
	)
	if toolCallMode && toolEnc == nil {
		if toolEnc, err = encoding.PredefinedEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
			return nil, nil, err
		}
		i.SetEncoder(toolEnc)
	} else if enc == nil {
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
		if !toolCallMode {
			enc.EnableValidate()
		}
	}
	contentCh := make(chan string)
	toolCh := make(chan any)
	outputCh := make(chan instructor.StreamData)
	go func() {
		defer close(contentCh)
		defer close(toolCh)
		defer close(outputCh)
		for item := range ch {
			outputCh <- item
			if toolCallMode {
				if item.Type == instructor.ToolStream {
					tValue := reflect.New(reflectType)
					instance := tValue.Interface()
					if err := toolEnc.Unmarshal([]byte(item.Content), instance); err == nil {
						toolCh <- instance
					}
				}
			} else if item.Type == instructor.ContentStream {
				contentCh <- item.Content
			}
		}
	}()
	if toolCallMode {
		return toolCh, outputCh, nil
	}
	parsedChan := enc.Read(ctx, contentCh)

	return parsedChan, outputCh, nil
}
