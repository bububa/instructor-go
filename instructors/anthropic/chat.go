package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	anthropic "github.com/liushuangls/go-anthropic/v2"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) Chat(ctx context.Context, request *anthropic.MessagesRequest, responseType any, response *anthropic.MessagesResponse) error {
	return chat.Handler(i, ctx, request, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.completionToolCall(ctx, *request, schema, response)
	case instructor.ModeJSONSchema, instructor.ModeJSON:
		return i.completionJSONSchema(ctx, *request, schema, response)
	default:
		return "", fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) completionToolCall(ctx context.Context, request anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (string, error) {
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

func (i *Instructor) completionJSONSchema(ctx context.Context, request anthropic.MessagesRequest, schema *instructor.Schema, response *anthropic.MessagesResponse) (string, error) {
	system := fmt.Sprintf(`
Please responsd with json in the following json_schema:

%s

Make sure to return an instance of the JSON, not the schema itself.
`, schema.String)

	request.Stream = false
	if request.System == "" {
		request.System = system
	} else {
		request.System += system
	}

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.CreateMessages(ctx, request)
	if err != nil {
		return "", err
	}

	text := resp.Content[0].Text
	if response != nil {
		*response = resp
	}
	return *text, nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *anthropic.MessagesResponse, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = anthropic.MessagesResponse{
		Usage: anthropic.MessagesUsage{
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
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
	response.Usage.InputTokens = usage.InputTokens
	response.Usage.OutputTokens = usage.OutputTokens
}

func (i *Instructor) CountUsageFromResponse(response *anthropic.MessagesResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	usage.InputTokens += response.Usage.InputTokens
	usage.OutputTokens += response.Usage.OutputTokens
}
