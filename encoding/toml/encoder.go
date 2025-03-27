package toml

import (
	"bytes"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/go-playground/validator/v10"

	"github.com/bububa/instructor-go"
)

var (
	IGNORE_PREFIX = []byte("```toml")
	IGNORE_SUFFIX = []byte("```")
)

type Encoder struct {
	reqType reflect.Type
}

func NewEncoder(req any) *Encoder {
	t := reflect.TypeOf(req)
	return &Encoder{
		reqType: t,
	}
}

func (e *Encoder) Marshal(v any) ([]byte, error) {
	return toml.Marshal(v)
}

func (e *Encoder) Unmarshal(bs []byte, ret any) error {
	data := cleanup(bs)
	return toml.Unmarshal(data, ret)
}

func (e *Encoder) Validate(req any) error {
	validate := validator.New()
	return validate.Struct(req)
}

func (e *Encoder) Context() []byte {
	tValue := reflect.New(e.reqType)
	instance := tValue.Interface()
	if f, ok := tValue.Elem().Interface().(instructor.Faker); ok {
		instance = f.Fake()
	} else {
		gofakeit.Struct(instance)
	}
	bs, err := e.Marshal(instance)
	if err != nil {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("\nPlease respond with TOML in the following TOML schema:\n\n")
	b.WriteString("```toml\n")
	b.Write(bs)
	b.WriteString("```")
	b.WriteString("\nMake sure to return an instance of the TOML, not the schema itself\n")
	return b.Bytes()
}

// cleanup the JSON by trimming prefixes and postfixes
func cleanup(bs []byte) []byte {
	// 找到 "```yaml" 的位置
	startIndex := bytes.Index(bs, IGNORE_PREFIX)
	if startIndex == -1 {
		// 如果没有找到起始标记，直接返回原始字符串
		return bs
	}

	// 计算移除起始标记及其之前的内容后的字符串
	contentAfterStart := bs[startIndex+len(IGNORE_PREFIX):]

	// 找到最后一个 "```" 的位置
	endIndex := bytes.LastIndex(contentAfterStart, IGNORE_SUFFIX)
	if endIndex == -1 {
		// 如果没有找到结束标记，直接返回从起始标记之后的内容
		return contentAfterStart
	}

	// 截取中间的有效内容
	result := contentAfterStart[:endIndex]

	return bytes.TrimSpace(result)
}
