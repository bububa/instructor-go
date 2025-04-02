package openai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bububa/instructor-go/encoding"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
) (stream <-chan string, thinking <-chan string, err error) {
	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType); err != nil {
				return nil, nil, err
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
	stream, thinking, err = i.createStream(ctx, &req, response)
	if err != nil {
		return nil, nil, err
	}
	return stream, thinking, err
}
