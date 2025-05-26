package anthropic

import (
	"bytes"
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
) (<-chan any, <-chan instructor.StreamData, error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan instructor.StreamData, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema:
		return i.chatSchemaStream(ctx, *request, response)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan instructor.StreamData, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
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

func (i *Instructor) chatSchemaStream(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan instructor.StreamData, error) {
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

func (i *Instructor) createStream(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (<-chan instructor.StreamData, error) {
	request.Stream = true
	if thinking := i.ThinkingConfig(); thinking != nil {
		request.Thinking = &anthropic.Thinking{
			BudgetTokens: thinking.Budget,
		}
		if thinking.Enabled {
			request.Thinking.Type = anthropic.ThinkingTypeEnabled
		} else {
			request.Thinking.Type = anthropic.ThinkingTypeDisabled
		}
	}
	toolCall := len(request.Tools) > 0
	ch := make(chan instructor.StreamData)
	sb := new(bytes.Buffer)
	var toolUse *instructor.ToolCall
	streamReq := anthropic.MessagesStreamRequest{
		MessagesRequest: *request,
		OnContentBlockStart: func(data anthropic.MessagesEventContentBlockStartData) {
			if tool := data.ContentBlock.MessageContentToolUse; tool != nil {
				toolUse = new(instructor.ToolCall)
				toolUse.ID = tool.ID
				toolUse.Name = tool.Name
			}
		},
		OnContentBlockStop: func(data anthropic.MessagesEventContentBlockStopData, _ anthropic.MessageContent) {
			if toolUse != nil {
//				ch <- instructor.StreamData{Type: instructor.ToolStream, ToolCall: toolUse}
			}
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			if thinking := data.Delta.MessageContentThinking; thinking != nil {
				ch <- instructor.StreamData{Type: instructor.ThinkingStream, Content: thinking.Thinking}
			}
			if toolCall {
				if partial := data.Delta.PartialJson; partial != nil {
					if i.Verbose() {
						sb.WriteString(*partial)
					}
					if toolUse != nil {
						toolUse.Content += *partial
					}
				}
			} else if text := data.Delta.Text; text != nil {
				if i.Verbose() {
					sb.WriteString(*text)
				}
				ch <- instructor.StreamData{Type: instructor.ContentStream, Content: *text}
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
		fmt.Fprintf(sb, "%s Response: \n", i.Provider())
	}
	go func() {
		defer close(ch)
		if i.Verbose() {
			defer func() {
				log.Println(sb.String())
			}()
		}
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
