package cohere

import (
	"context"

	cohere "github.com/cohere-ai/cohere-go/v2"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	response *cohere.NonStreamedChatResponse,
) (<-chan string, error) {
	stream, err := i.createStream(ctx, request, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}
