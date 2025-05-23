package encoding

import (
	"errors"

	"github.com/bububa/instructor-go"

	dummyenc "github.com/bububa/instructor-go/encoding/dummy"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	tomlenc "github.com/bububa/instructor-go/encoding/toml"
	yamlenc "github.com/bububa/instructor-go/encoding/yaml"
)

func PredefinedEncoder(mode instructor.Mode, req any, namer instructor.SchemaNamer) (instructor.Encoder, error) {
	var (
		enc instructor.Encoder
		err error
	)
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err = jsonenc.NewEncoder(req, namer)
	case instructor.ModeYAML:
		enc = yamlenc.NewEncoder(req)
	case instructor.ModeTOML:
		enc = tomlenc.NewEncoder(req)
	case instructor.ModePlainText:
		enc = dummyenc.NewEncoder()
	default:
		return nil, errors.New("no predefined encoder")
	}
	return enc, err
}

func PredefinedStreamEncoder(mode instructor.Mode, req any, namer instructor.SchemaNamer) (instructor.StreamEncoder, error) {
	var (
		enc instructor.StreamEncoder
		err error
	)
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err = jsonenc.NewStreamEncoder(req, false, namer)
	case instructor.ModeYAML:
		enc, err = yamlenc.NewStreamEncoder(req)
	case instructor.ModeTOML:
		enc, err = tomlenc.NewStreamEncoder(req)
	case instructor.ModePlainText:
		enc = dummyenc.NewStreamEncoder()
	default:
		return nil, errors.New("no predefined encoder")
	}
	return enc, err
}
