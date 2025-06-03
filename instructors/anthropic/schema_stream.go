package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/mark3labs/mcp-go/mcp"

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
	return i.createStream(ctx, request, response, false)
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
	return i.createStream(ctx, request, response, false)
}

func (i *Instructor) createStream(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse, toolRequest bool) (<-chan instructor.StreamData, error) {
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
	if !toolRequest {
		i.InjectMCP(ctx, &request)
	}
	ch := make(chan instructor.StreamData)
	sb := new(bytes.Buffer)
	toolCallMap := make(map[int]anthropic.MessageContentToolUse)
	toolUseInput := make(map[int]string)
	streamReq := anthropic.MessagesStreamRequest{
		MessagesRequest: request,
		OnContentBlockStart: func(data anthropic.MessagesEventContentBlockStartData) {
			if data.ContentBlock.Type == anthropic.MessagesContentTypeToolUse {
				if tool := data.ContentBlock.MessageContentToolUse; tool != nil {
					toolCallMap[data.Index] = anthropic.MessageContentToolUse{
						ID:   tool.ID,
						Name: tool.Name,
					}
				}
			}
		},
		OnContentBlockDelta: func(data anthropic.MessagesEventContentBlockDeltaData) {
			switch data.Delta.Type {
			case anthropic.MessagesContentTypeToolUse:
				if tool := data.Delta.MessageContentToolUse; tool != nil {
					if _, ok := toolCallMap[data.Index]; ok {
						if partial := data.Delta.PartialJson; partial != nil {
							toolUseInput[data.Index] += *partial
						}
					}
				}
			case anthropic.MessagesContentTypeText:
				if thinking := data.Delta.MessageContentThinking; thinking != nil {
					ch <- instructor.StreamData{Type: instructor.ThinkingStream, Content: thinking.Thinking}
				} else if text := data.Delta.Text; text != nil {
					if i.Verbose() {
						sb.WriteString(*text)
					}
					ch <- instructor.StreamData{Type: instructor.ContentStream, Content: *text}
				}
			}
		},
		OnMessageDelta: func(data anthropic.MessagesEventMessageDeltaData) {
			if response != nil {
				response.Usage.InputTokens += data.Usage.InputTokens
				response.Usage.OutputTokens += data.Usage.OutputTokens
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
		defer close(ch)
		if i.Verbose() && !toolRequest {
			defer func() {
				fmt.Fprintf(sb, "%s Response: \n", i.Provider())
				log.Println(sb.String())
			}()
		}
		var usage anthropic.MessagesUsage
		defer func() {
			if response != nil {
				usage = response.Usage
			}
			if i.memory != nil && !toolRequest && sb.String() != "" {
				i.memory.Add(anthropic.NewAssistantTextMessage(sb.String()))
			}
		}()
		defer func() {
			toolCalls := make([]anthropic.MessageContentToolUse, 0, len(toolCallMap))
			contents := make([]anthropic.MessageContent, 0, len(toolCallMap))
			for idx, toolCall := range toolCallMap {
				input, ok := toolUseInput[idx]
				if !ok {
					continue
				}
				toolCall.Input = []byte(input)
				toolCalls = append(toolCalls, toolCall)
				contents = append(contents, anthropic.NewToolUseMessageContent(toolCall.ID, toolCall.Name, toolCall.Input))
			}
			if len(toolCalls) == 0 {
				return
			}
			oldMessageCount := len(request.Messages)
			request.Messages = append(request.Messages, anthropic.Message{
				Role:    anthropic.RoleAssistant,
				Content: contents,
			})
			messageContents := make([]anthropic.MessageContent, 0, len(toolCalls))
			for _, toolCall := range toolCalls {
				content, call := i.CallMCP(ctx, &toolCall)
				if call != nil {
					ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: call}
				} else {
					var shouldReturn bool
					for _, tool := range request.Tools {
						if tool.Name == toolCall.Name {
							callReq := new(mcp.CallToolRequest)
							callReq.Params.Name = toolCall.Name
							callReq.Params.Arguments = toolCall.Input
							call := instructor.ToolCall{
								Request: callReq,
							}
							ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: &call}
							shouldReturn = true
						}
					}
					if shouldReturn {
						return
					}
				}
				messageContents = append(messageContents, content)
			}
			request.Messages = append(request.Messages, anthropic.Message{
				Role:    anthropic.RoleUser,
				Content: messageContents,
			})
			if newMessageCount := len(request.Messages); newMessageCount > oldMessageCount && i.memory != nil {
				i.memory.Add(request.Messages[oldMessageCount-1 : newMessageCount]...)
			}
			tmpCh, err := i.createStream(ctx, request, response, true)
			if err != nil {
				ch <- instructor.StreamData{Type: instructor.ErrorStream, Err: err}
				return
			}
			for v := range tmpCh {
				if i.Verbose() && v.Type == instructor.ContentStream {
					sb.WriteString(v.Content)
				}
				ch <- v
			}
			if response != nil {
				response.Usage.InputTokens += usage.InputTokens
				response.Usage.OutputTokens += usage.OutputTokens
			}
		}()
		resp, err := i.CreateMessagesStream(ctx, streamReq)
		if response != nil {
			*response = resp
		}
		if err != nil {
			ch <- instructor.StreamData{Type: instructor.ErrorStream, Err: err}
		}
	}()

	return ch, nil
}
