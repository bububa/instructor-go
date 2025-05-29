package json

import (
	"bytes"
	"reflect"

	"github.com/bububa/ljson"
	"github.com/go-playground/validator/v10"

	"github.com/bububa/instructor-go"
)

type Encoder struct {
	schema *instructor.Schema
  reqType reflect.Type
}

func NewEncoder(req any, namer instructor.SchemaNamer) (*Encoder, error) {
	t := reflect.TypeOf(req)
	schema, err := instructor.NewSchema(t, namer)
	if err != nil {
		return nil, err
	}
	return &Encoder{
		schema: schema,
    reqType: t,
	}, nil
}

func (e *Encoder) Instance() any {
	tValue := reflect.New(e.reqType)
	return tValue.Interface()
}

func (e *Encoder) Elem() any {
	tValue := reflect.New(e.reqType)
	return tValue.Elem()
}


func (e *Encoder) Marshal(req any) ([]byte, error) {
	return []byte(e.schema.String), nil
}

func (e *Encoder) Unmarshal(bs []byte, ret any) error {
	data := cleanup(bs)
	return ljson.Unmarshal(data, ret)
}

func (e *Encoder) Validate(req any) error {
	validate := validator.New()
	return validate.Struct(req)
}

func (e *Encoder) Context() []byte {
	bs, err := e.Marshal(nil)
	if err != nil {
		return nil
	}
	var b bytes.Buffer
	b.WriteString("\nPlease respond with JSON in the following JSON schema:\n")
	b.WriteString("```json\n")
	b.Write(bs)
	b.WriteString("\n```")
	b.WriteString("Make sure to return an instance of the JSON, not the schema itself\n")
	return b.Bytes()
}

func (e *Encoder) Schema() *instructor.Schema {
	return e.schema
}

// cleanup the JSON by trimming prefixes and postfixes
func cleanup(bs []byte) []byte {
	trimmedPrefix := trimPrefixBeforeJSON(bs)
	trimmedJSON := trimPostfixAfterJSON(trimmedPrefix)
  return trimmedJSON
}

// Removes any prefixes before the JSON (like "Sure, here you go:")
func trimPrefixBeforeJSON(bs []byte) []byte {
	startObject := bytes.IndexByte(bs, '{')
	startArray := bytes.IndexByte(bs, '[')

	var start int
	if startObject == -1 && startArray == -1 {
		return bs // No opening brace or bracket found, return the original string
	} else if startObject == -1 {
		start = startArray
	} else if startArray == -1 {
		start = startObject
	} else {
		start = min(startObject, startArray)
	}

	return bs[start:]
}

// Removes any postfixes after the JSON
func trimPostfixAfterJSON(bs []byte) []byte {
	endObject := bytes.LastIndexByte(bs, '}')
	endArray := bytes.LastIndexByte(bs, ']')

	var end int
	if endObject == -1 && endArray == -1 {
		return bs // No closing brace or bracket found, return the original string
	} else if endObject == -1 {
		end = endArray
	} else if endArray == -1 {
		end = endObject
	} else {
		end = max(endObject, endArray)
	}

	return bs[:end+1]
}
