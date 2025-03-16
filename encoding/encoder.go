package encoding

import (
	"errors"

	"github.com/bububa/instructor-go"

	jsonenc "github.com/bububa/instructor-go/encoding/json"
)

func PredefinedEncoder(mode instructor.Mode, req any) (instructor.Encoder, error) {
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err := jsonenc.NewEncoder(req)
		if err != nil {
			return nil, err
		}
		return enc, nil
	}
	return nil, errors.New("no predefined encoder")
}

func PredefinedStreamEncoder(mode instructor.Mode, req any) (instructor.StreamEncoder, error) {
	switch mode {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict, instructor.ModeJSON, instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		enc, err := jsonenc.NewStreamEncoder(req, false)
		if err != nil {
			return nil, err
		}
		return enc, nil
	}
	return nil, errors.New("no predefined encoder")
}
