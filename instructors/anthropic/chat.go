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

func (i *Instructor) Chat(ctx context.Context, request *anthropic.MessagesRequest, responseType any, response *anthropic.MessagesResponse) error {
	return chat.Handler(i, ctx, request, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *anthropic.MessagesRequest, response *anthropic.MessagesResponse) (string, error) {
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
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.completionToolCall(ctx, *request, response)
	default:
		return i.completion(ctx, *request, response)
	}
}

func (i *Instructor) completionToolCall(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	request.Stream = false
	request.Tools = []anthropic.ToolDefinition{}

	for _, function := range schema.Functions {
		t := anthropic.ToolDefinition{
			Name:        function.Name,
			Description: function.Description,
			InputSchema: function.Parameters,
		}
		request.Tools = append(request.Tools, t)
	}

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.CreateMessages(ctx, request)
	if err != nil {
		return "", err
	}

	for _, c := range resp.Content {
		if c.Type != anthropic.MessagesContentTypeToolUse {
			// Skip non tool responses
			continue
		}

		toolInput, err := json.Marshal(c.Input)
		if err != nil {
			i.EmptyResponseWithResponseUsage(response, &resp)
			return "", err
		}
		// TODO: handle more than 1 tool use
		if response != nil {
			*response = resp
		}
		return string(toolInput), nil
	}

	i.EmptyResponseWithResponseUsage(response, &resp)
	return "", errors.New("more than 1 tool response at a time is not implemented")
}

func (i *Instructor) completion(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (string, error) {
	request.Stream = false
	if bs := i.Encoder().Context(); bs != nil {
		if request.System == "" {
			request.System = string(bs)
		} else {
			request.System = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", request.System, bs)
		}
	}
	return i.chatCompletionWrapper(ctx, request, response)
}

func (i *Instructor) chatCompletionWrapper(ctx context.Context, request anthropic.MessagesRequest, response *anthropic.MessagesResponse) (string, error) {
	i.InjectMCP(ctx, &request)
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	resp, err := i.CreateMessages(ctx, request)
	if err != nil {
		return "", err
	}
	if i.Verbose() {
		bs, _ := json.MarshalIndent(resp, "", "  ")
		log.Printf("%s Response: %s\n", i.Provider(), string(bs))
	}
	if response != nil {
		*response = resp
	}
	var messageContents []anthropic.MessageContent
	for _, c := range resp.Content {
		if c.Type == anthropic.MessagesContentTypeToolUse {
			messageContent, call := i.CallMCP(ctx, c.MessageContentToolUse)
			if i.Verbose() && call != nil {
				bs, _ := json.MarshalIndent(call, "", "  ")
				log.Printf("%s ToolCall Result: %s\n", i.Provider(), string(bs))
			}
			messageContents = append(messageContents, messageContent)
		}
	}
	if len(messageContents) > 0 {
		var usage anthropic.MessagesUsage
		if response != nil {
			usage = response.Usage
		}
		request.Messages = append(request.Messages,
			anthropic.Message{
				Role:    anthropic.RoleAssistant,
				Content: resp.Content,
			},
			anthropic.Message{
				Role:    anthropic.RoleUser,
				Content: messageContents,
			})
		text, err := i.chatCompletionWrapper(ctx, request, response)
		if response != nil {
			response.Usage.InputTokens += usage.InputTokens
			response.Usage.OutputTokens += usage.OutputTokens
		}
		return text, err
	}
	text := resp.Content[0].Text
	return *text, nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *anthropic.MessagesResponse, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = anthropic.MessagesResponse{
		Usage: anthropic.MessagesUsage{
			InputTokens:  int(usage.InputTokens),
			OutputTokens: int(usage.OutputTokens),
		},
	}
}

func (i *Instructor) EmptyResponseWithResponseUsage(ret *anthropic.MessagesResponse, response *anthropic.MessagesResponse) {
	if ret == nil {
		return
	}
	if response == nil {
		*ret = anthropic.MessagesResponse{}
		return
	}
	*ret = anthropic.MessagesResponse{
		Usage: response.Usage,
	}
}

func (i *Instructor) SetUsageSumToResponse(response *anthropic.MessagesResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	response.Usage.InputTokens = int(usage.InputTokens)
	response.Usage.OutputTokens = int(usage.OutputTokens)
}

func (i *Instructor) CountUsageFromResponse(response *anthropic.MessagesResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	usage.InputTokens += int64(response.Usage.InputTokens)
	usage.OutputTokens += int64(response.Usage.OutputTokens)
}
