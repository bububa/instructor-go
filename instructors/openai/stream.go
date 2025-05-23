package openai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
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
		for idx, msg := range req.Messages {
			if msg.Role == "system" {
				bs := i.Encoder().Context()
				if bs != nil {
					req.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, string(bs))
				}
			}
		}
		if _, ok := i.Encoder().(*jsonenc.Encoder); ok {
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
		} else {
			req.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeText}
		}
	}
	return i.createStream(ctx, &req, response)
}
