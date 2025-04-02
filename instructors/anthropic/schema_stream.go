package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *anthropic.MessagesRequest,
	responseType any,
	response *anthropic.MessagesResponse,
) (stream <-chan any, thinkingStream <-chan string, err error) {
	stream, thinkingStream, err = chat.SchemaStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, nil, err
	}

	return stream, thinkingStream, err
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan string, <-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema:
		return i.chatSchemaStream(ctx, *request, response)
	default:
		return nil, nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan string, <-chan string, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, nil, errors.New("encoder must be JSON Encoder")
	}
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

func (i *Instructor) chatSchemaStream(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan string, <-chan string, error) {
	request.Stream = true
	if bs := i.StreamEncoder().Context(); bs != nil {
		if request.System == "" {
			request.System = string(bs)
		} else {
			request.System = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", request.System, bs)
		}
	}
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) createStream(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan string, <-chan string, error) {
	request.Stream = true
	toolCall := len(request.Tools) > 0
	ch := make(chan string)
	thinkingCh := make(chan string)
	streamReq := anthropic.MessagesStreamRequest{
		MessagesRequest: *request,
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if thinking := data.Delta.MessageContentThinking; thinking != nil {
				thinkingCh <- thinking.Thinking
			}
			if toolCall {
				if partial := data.Delta.PartialJson; partial != nil {
					ch <- *partial
				}
			} else if text := data.Delta.Text; text != nil {
				ch <- *text
			}
		},
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		request.Thinking = &anthropic.Thinking{
			BudgetTokens: thinkingConfig.Budget,
			Type:         anthropic.ThinkingTypeEnabled,
		}
	}
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	go func() {
		defer close(thinkingCh)
		defer close(ch)
		resp, err := i.CreateMessagesStream(ctx, streamReq)
		if err != nil {
			return
		}
		if response != nil {
			*response = resp
		}
	}()

	return ch, thinkingCh, nil
}
