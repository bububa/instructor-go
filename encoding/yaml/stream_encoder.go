package yaml

import (
	"bufio"
	"bytes"
	"context"
	"reflect"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
)

type StreamEncoder struct {
	reqType  reflect.Type
	buffer   *bytes.Buffer
	validate bool
}

func NewStreamEncoder(req any) (*StreamEncoder, error) {
	t := reflect.TypeOf(req)
	buffer := new(bytes.Buffer)
	return &StreamEncoder{
		reqType: t,
		buffer:  buffer,
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
	b.WriteString("\nPlease respond with a list where the elements following YAML schema which is seperated by '----' for each elements:\n\n")
	for range 3 {
		b.WriteString("----\n")
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
	b.WriteString("\n----\n")
	b.WriteString("\nDo not output anything else except the list.\n")
	// b.WriteString("\nMake sure to return a list with the elements an instance of the YAML, not the schema itself.\n")
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
						bs := bytes.TrimSuffix(bytes.TrimPrefix(bytes.TrimSpace(e.buffer.Bytes()), IGNORE_PREFIX), IGNORE_SUFFIX)
						instance := reflect.New(e.reqType).Interface()
						if err := yaml.Unmarshal(bs, instance); err == nil {
							if e.validate {
								// Validate the instance
								if err := e.Validate(instance); err != nil {
									return
								}
							}
							parsedChan <- instance
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
	scanner := bufio.NewScanner(e.buffer)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\n'); i >= 0 {
			// 包括换行符 \n
			return i + 1, data[0 : i+1], nil
		}
		// 如果没有找到换行符，返回整个剩余数据
		if atEOF {
			return len(data), data, nil
		}
		// 请求更多数据
		return 0, nil, nil
	})
	for scanner.Scan() {
		bs := scanner.Bytes()
		if trimmed := bytes.TrimSpace(bs); internal.IsAllSameByte(trimmed, '-') && len(trimmed) > 1 {
			if block.Len() > 0 {
				in := bytes.TrimSuffix(bytes.TrimPrefix(bytes.TrimSpace(block.Bytes()), IGNORE_PREFIX), IGNORE_SUFFIX)
				instance := reflect.New(e.reqType).Interface()
				if err := yaml.Unmarshal(in, instance); err == nil {
					if e.validate {
						// Validate the instance
						if err := e.Validate(instance); err != nil {
							block.Reset()
							continue
						}
					}
					parsedChan <- instance
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
