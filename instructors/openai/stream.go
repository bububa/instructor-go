package openai

import (
	"context"

	openai "github.com/sashabaranov/go-openai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	response *openai.ChatCompletionResponse,
) (stream <-chan string, err error) {
	stream, err = i.createStream(ctx, request, response)
	if err != nil {
		return nil, err
	}
	return stream, err
}
