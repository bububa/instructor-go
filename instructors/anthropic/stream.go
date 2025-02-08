package anthropic

import (
	"context"

	anthropic "github.com/liushuangls/go-anthropic/v2"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *anthropic.MessagesRequest,
	response *anthropic.MessagesResponse,
) (stream <-chan string, err error) {
	stream, err = i.createStream(ctx, request, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}
