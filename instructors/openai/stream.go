package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *openai.ChatCompletionNewParams,
	responseType any,
	response *openai.ChatCompletion,
) (<-chan instructor.StreamData, error) {
	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
				return nil, err
			} else {
				i.SetEncoder(enc)
			}
		}
		var (
			hasSystem bool
			lastIdx   = -1
		)
		for idx, msg := range req.Messages {
			if system := msg.OfSystem; system != nil {
				bs := i.Encoder().Context()
				if bs != nil {
					system.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", system.Content.OfString.Value, string(bs)))
					req.Messages[idx] = msg
					hasSystem = true
				}
			}
		}
		if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
			if i.Mode() == instructor.ModeJSONStrict {
				schema := enc.Schema()
				structName := schema.NameFromRef()
				schemaWrapper := ResponseFormatSchemaWrapper{
					Type:        "object",
					Required:    []string{structName},
					Definitions: &schema.Definitions,
					Properties: &jsonschema.Definitions{
						structName: schema.Definitions[structName],
					},
					AdditionalProperties: false,
				}

				schemaJSON, _ := json.Marshal(schemaWrapper)
				schemaRaw := json.RawMessage(schemaJSON)

				req.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
					OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
						JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
							Name:        structName,
							Description: openai.String(schema.Description),
							Schema:      schemaRaw,
							Strict:      openai.Bool(true),
						},
					},
				}
			} else {
				if !hasSystem && lastIdx >= 0 {
					bs := i.StreamEncoder().Context()
					if msg := req.Messages[lastIdx].OfUser; msg != nil {
						req.Messages[lastIdx].OfUser.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content.OfString.Value, string(bs)))
					}
				}
				req.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
					OfJSONObject: new(openai.ResponseFormatJSONObjectParam),
				}
			}
		} else {
			request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfText: new(openai.ResponseFormatTextParam),
			}
		}
	}
	return i.createStream(ctx, req, response, false)
}
