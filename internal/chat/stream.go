package chat

import (
	"context"
	"encoding/json"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

const WRAPPER_END = `"items": [`

func SchemaStreamHandler[T any, RESP any](i instructor.SchemaStreamInstructor[T, RESP], ctx context.Context, request *T, responseType any, resp *RESP) (<-chan any, <-chan instructor.StreamData, error) {
	var (
		isToolCall = i.Mode() == instructor.ModeToolCall || i.Mode() == instructor.ModeToolCallStrict
		enc        = i.StreamEncoder()
		err        error
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
	if isToolCall {
		parsedChan := make(chan any)
		itemEnc := i.Encoder()
		if itemEnc == nil {
			if itemEnc, err = encoding.PredefinedEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
				return nil, nil, err
			}
			i.SetEncoder(itemEnc)
		}
		list := struct {
			Items []any `json:"items,omitempty"`
		}{}
		go func() {
			defer close(contentCh)
			defer close(outputCh)
			defer close(parsedChan)
			for item := range ch {
				outputCh <- item
				if item.Type == instructor.ToolCallStream && item.ToolCall != nil && item.ToolCall.Request != nil {
					if bs, err := json.Marshal(item.ToolCall.Request.Params.Arguments); err == nil {
						if err := itemEnc.Unmarshal(bs, &list); err == nil {
							instance := itemEnc.Instance()
							for _, v := range list.Items {
								if bs, err := json.Marshal(v); err != nil {
									continue
								} else if err := json.Unmarshal(bs, instance); err == nil {
									parsedChan <- instance
								}
							}
						}
					}
				}
			}
		}()
		return parsedChan, outputCh, nil
	}
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
