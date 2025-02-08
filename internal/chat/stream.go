package chat

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
)

type StreamWrapper[T any] struct {
	Items []T `json:"items"`
}

const WRAPPER_END = `"items": [`

func JSONStreamHandler[T any, RESP any](i instructor.JSONStreamInstructor[T, RESP], ctx context.Context, request *T, responseType any, resp *RESP) (<-chan interface{}, error) {
	t := reflect.TypeOf(responseType)

	streamWrapperType := reflect.StructOf([]reflect.StructField{
		{
			Name:      "Items",
			Type:      reflect.SliceOf(t),
			Tag:       `json:"items"`,
			Anonymous: false,
		},
	})

	schema, err := instructor.NewSchema(streamWrapperType)
	if err != nil {
		return nil, err
	}

	ch, err := i.JSONStreamHandler(ctx, request, schema, resp)
	if err != nil {
		return nil, err
	}

	var validate *validator.Validate
	shouldValidate := i.Validate()
	if shouldValidate {
		validate = validator.New()
	}

	parsedChan := parseStream(ctx, ch, validate, t)

	return parsedChan, nil
}

func parseStream(ctx context.Context, ch <-chan string, validate *validator.Validate, responseType reflect.Type) <-chan interface{} {
	parsedChan := make(chan any)

	go func() {
		defer close(parsedChan)

		buffer := new(strings.Builder)
		inArray := false

		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-ch:
				if !ok {
					// Stream closed
					processRemainingBuffer(buffer, parsedChan, validate, responseType)
					return
				}

				buffer.WriteString(text)

				// Eat all input until elements stream starts
				if !inArray {
					inArray = startArray(buffer)
				}

				processBuffer(buffer, parsedChan, validate, responseType)
			}
		}
	}()

	return parsedChan
}

func startArray(buffer *strings.Builder) bool {
	data := buffer.String()

	idx := strings.Index(data, WRAPPER_END)
	if idx == -1 {
		return false
	}

	trimmed := strings.TrimSpace(data[idx+len(WRAPPER_END):])
	buffer.Reset()
	buffer.WriteString(trimmed)

	return true
}

func processBuffer(buffer *strings.Builder, parsedChan chan<- interface{}, validate *validator.Validate, responseType reflect.Type) {
	data := buffer.String()

	data, remaining := internal.GetFirstFullJSONElement(&data)

	decoder := json.NewDecoder(strings.NewReader(data))

	for decoder.More() {
		instance := reflect.New(responseType).Interface()
		err := decoder.Decode(instance)
		if err != nil {
			break
		}

		if validate != nil {
			// Validate the instance
			err = validate.Struct(instance)
			if err != nil {
				break
			}
		}

		parsedChan <- instance

		buffer.Reset()
		buffer.WriteString(remaining)
	}
}

func processRemainingBuffer(buffer *strings.Builder, parsedChan chan<- interface{}, validate *validator.Validate, responseType reflect.Type) {
	data := buffer.String()

	data = internal.ExtractJSON(&data)

	if idx := strings.LastIndex(data, "]"); idx != -1 {
		data = data[:idx]
	}

	processBuffer(buffer, parsedChan, validate, responseType)
}
