package encoding

import (
	"errors"

	"github.com/bububa/instructor-go"

	dummyenc "github.com/bububa/instructor-go/encoding/dummy"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	yamlenc "github.com/bububa/instructor-go/encoding/yaml"
)

func PredefinedEncoder(mode instructor.Mode, req any) (instructor.Encoder, error) {
	var (
		enc instructor.Encoder
		err error
	)
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err = jsonenc.NewEncoder(req)
	case instructor.ModeYAML:
		enc = yamlenc.NewEncoder()
	case instructor.ModePlainText:
		enc = dummyenc.NewEncoder()
	default:
		return nil, errors.New("no predefined encoder")
	}
	return enc, err
}

func PredefinedStreamEncoder(mode instructor.Mode, req any) (instructor.StreamEncoder, error) {
	var (
		enc instructor.StreamEncoder
		err error
	)
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err = jsonenc.NewStreamEncoder(req, false)
	case instructor.ModeYAML:
		enc, err = yamlenc.NewStreamEncoder(req)
	case instructor.ModePlainText:
		enc = dummyenc.NewStreamEncoder()
	default:
		return nil, errors.New("no predefined encoder")
	}
	return enc, err
}
