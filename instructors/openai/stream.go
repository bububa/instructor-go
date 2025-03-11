package openai

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bububa/instructor-go"
	openai "github.com/sashabaranov/go-openai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
) (stream <-chan string, err error) {
	if (i.Mode() == instructor.ModeJSON || i.Mode() == instructor.ModeJSONSchema) && responseType != nil {
		t := reflect.TypeOf(responseType)

		schema, err := instructor.NewSchema(t)
		if err != nil {
			return nil, err
		}
		for idx, msg := range request.Messages {
			if msg.Role == "system" {
				request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, appendJSONMessage(schema))
			}
		}
	}
	stream, err = i.createStream(ctx, request, response)
	if err != nil {
		return nil, err
	}
	return stream, err
}
