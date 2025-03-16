package yaml

import (
	"bufio"
	"bytes"
	"context"
	"reflect"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/bububa/instructor-go"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type StreamEncoder struct {
	reqType  reflect.Type
	buffer   *bytes.Buffer
	scanner  *bufio.Scanner
	validate bool
}

func NewStreamEncoder(req any) (*StreamEncoder, error) {
	t := reflect.TypeOf(req)
	buffer := new(bytes.Buffer)
	return &StreamEncoder{
		reqType: t,
		buffer:  buffer,
		scanner: bufio.NewScanner(buffer),
	}, nil
}

func (e *StreamEncoder) EnableValidate() {
	e.validate = true
}

func (e *StreamEncoder) Validate(req any) error {
	validate := validator.New()
	return validate.Struct(req)
}

func (e *StreamEncoder) Marshal(req any) ([]byte, error) {
	return yaml.Marshal(req)
}

func (e *StreamEncoder) Context() []byte {
	var b bytes.Buffer
	b.WriteString("\nPlease respond with a YAML array where the elements following YAML schema which is seperated by a blank line for each elements:\n")
	b.WriteString("```yaml\n")
	for i := range 3 {
		if i > 0 {
			b.WriteString("\n\n")
		}
		instance := reflect.New(e.reqType).Interface()
		if f, ok := instance.(instructor.Faker); ok {
			instance = f.Fake()
		} else {
			gofakeit.Struct(instance)
		}
		bs, err := e.Marshal(instance)
		if err != nil {
			return nil
		}
		b.Write(bs)
	}
	b.WriteString("\n```")
	b.WriteString("Make sure to return an array with the elements an instance of the YAML, not the schema itself.\n")
	return b.Bytes()
}

func (e *StreamEncoder) Read(ctx context.Context, ch <-chan string) <-chan any {
	parsedChan := make(chan any)
	e.buffer.Reset()
	go func() {
		defer close(parsedChan)
		defer e.buffer.Reset()
		for {
			select {
			case <-ctx.Done():
				return
			case text, ok := <-ch:
				if !ok {
					// Stream closed
					if e.buffer.Len() > 0 {
						bs := bytes.TrimSpace(e.buffer.Bytes())
						instance := reflect.New(e.reqType).Interface()
						if err := yaml.Unmarshal(bs, instance); err == nil {
							if e.validate {
								// Validate the instance
								if err := e.Validate(instance); err == nil {
									parsedChan <- instance
								}
							}
						}
					}
					return
				}
				e.buffer.WriteString(text)
				e.processBuffer(parsedChan)
			}
		}
	}()
	return parsedChan
}

func (e *StreamEncoder) processBuffer(parsedChan chan<- any) {
	block := new(bytes.Buffer)
	for e.scanner.Scan() {
		bs := e.scanner.Bytes()
		if trimmed := bytes.TrimSpace(bs); len(trimmed) == 0 {
			if block.Len() > 0 {
				instance := reflect.New(e.reqType).Interface()
				if err := yaml.Unmarshal(block.Bytes(), instance); err == nil {
					if e.validate {
						// Validate the instance
						if err := e.Validate(instance); err == nil {
							parsedChan <- instance
						}
					}
				}
			}
			block.Reset()
		} else {
			block.Write(bs)
		}
	}
	e.buffer.Reset()
	if block.Len() > 0 {
		e.buffer.Write(block.Bytes())
	}
}
