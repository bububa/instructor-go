package anthropic

import (
	"context"
	"fmt"

	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) JSONStream(
	ctx context.Context,
	request *anthropic.MessagesRequest,
	responseType any,
	response *anthropic.MessagesResponse,
) (stream <-chan any, err error) {
	stream, err = chat.JSONStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) JSONStreamHandler(ctx context.Context, request *anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (<-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, schema, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, schema, response)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (<-chan string, error) {
	request.Stream = true
	request.Tools = []anthropic.ToolDefinition{}

	for _, function := range schema.Functions {
		t := anthropic.ToolDefinition{
			Name:        function.Name,
			Description: function.Description,
			InputSchema: function.Parameters,
		}
		request.Tools = append(request.Tools, t)
	}
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) chatJSONStream(ctx context.Context, request anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (<-chan string, error) {
	request.Stream = true
	system := fmt.Sprintf(`
Please respond with a JSON array where the elements following JSON schema:

%s

Make sure to return an array with the elements an instance of the JSON, not the schema itself.
`, schema.String)
	if request.System == "" {
		request.System = system
	} else {
		request.System += system
	}
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) createStream(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan string, error) {
	request.Stream = true
	toolCall := len(request.Tools) > 0
	ch := make(chan string)
	streamReq := anthropic.MessagesStreamRequest{
		MessagesRequest: *request,
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if toolCall {
				if partial := data.Delta.PartialJson; partial != nil {
					ch <- *partial
				}
			} else if text := data.Delta.Text; text != nil {
				ch <- *text
			}
		},
	}
	go func() {
		defer close(ch)
		resp, err := i.CreateMessagesStream(ctx, streamReq)
		if err != nil {
			return
		}
		if response != nil {
			*response = resp
		}
	}()

	return ch, nil
}
