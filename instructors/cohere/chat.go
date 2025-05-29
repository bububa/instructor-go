package cohere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) Chat(
	ctx context.Context,
	request *cohere.ChatRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) error {
	return chat.Handler(i, ctx, request, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *cohere.ChatRequest, response *cohere.NonStreamedChatResponse) (string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCall(ctx, *request, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema, instructor.ModeJSONStrict:
		return i.completion(ctx, *request, response)
	default:
		return "", fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCall(ctx context.Context, request cohere.ChatRequest, response *cohere.NonStreamedChatResponse) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	request.Tools = []*cohere.Tool{createCohereTools(schema)}

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.Client.Chat(ctx, &request)
	if response != nil {
		*response = *resp
	}
	if err != nil {
		return "", err
	}
	if len(resp.ToolCalls) > 0 {
		toolInput, err := json.Marshal(resp.ToolCalls[0].Parameters)
		if err != nil {
			i.EmptyResponseWithResponseUsage(response, resp)
			return "", err
		}
		// TODO: handle more than 1 tool use
		return string(toolInput), nil

	}
	i.EmptyResponseWithResponseUsage(response, resp)
	return "", errors.New("more than 1 tool response at a time is not implemented")
}

func (i *Instructor) completion(ctx context.Context, request cohere.ChatRequest, response *cohere.NonStreamedChatResponse) (string, error) {
	if bs := i.Encoder().Context(); bs != nil {
		if system := request.Preamble; system == nil {
			request.Preamble = internal.ToPtr(string(bs))
		} else {
			request.Preamble = internal.ToPtr(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", *system, string(bs)))
		}
	}
	return i.chat(ctx, request, response)
}

func (i *Instructor) chat(ctx context.Context, request cohere.ChatRequest, response *cohere.NonStreamedChatResponse) (string, error) {
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.Client.Chat(ctx, &request)
	if response != nil {
		*response = *resp
	}
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *cohere.NonStreamedChatResponse, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = cohere.NonStreamedChatResponse{
		Meta: &cohere.ApiMeta{
			Tokens: &cohere.ApiMetaTokens{
				InputTokens:  internal.ToPtr(float64(usage.InputTokens)),
				OutputTokens: internal.ToPtr(float64(usage.OutputTokens)),
			},
		},
	}
}

func (i *Instructor) EmptyResponseWithResponseUsage(ret *cohere.NonStreamedChatResponse, response *cohere.NonStreamedChatResponse) {
	if ret == nil {
		return
	}
	var resp cohere.NonStreamedChatResponse
	if response == nil {
		*ret = resp
		return
	}

	resp.Meta = new(cohere.ApiMeta)
	*resp.Meta = *response.Meta
	*ret = resp
}

func (i *Instructor) SetUsageSumToResponse(response *cohere.NonStreamedChatResponse, usage *instructor.UsageSum) {
	if response == nil {
		return
	}
	if response.Meta == nil {
		response.Meta = new(cohere.ApiMeta)
	}
	if response.Meta.Tokens == nil {
		response.Meta.Tokens = new(cohere.ApiMetaTokens)
	}
	response.Meta.Tokens.InputTokens = internal.ToPtr(float64(usage.InputTokens))
	response.Meta.Tokens.OutputTokens = internal.ToPtr(float64(usage.OutputTokens))
}

func (i *Instructor) CountUsageFromResponse(response *cohere.NonStreamedChatResponse, usage *instructor.UsageSum) {
	if response == nil || response.Meta == nil || response.Meta.Tokens == nil {
		return
	}
	if v := response.Meta.Tokens.InputTokens; v != nil {
		usage.InputTokens += int64(*v)
	}
	if v := response.Meta.Tokens.OutputTokens; v != nil {
		usage.OutputTokens += int64(*v)
	}
}

func createCohereTools(schema *instructor.Schema) *cohere.Tool {
	tool := &cohere.Tool{
		Name:                 "functions",
		Description:          schema.Description,
		ParameterDefinitions: make(map[string]*cohere.ToolParameterDefinitionsValue),
	}

	for _, function := range schema.Functions {
		parameterDefinition := &cohere.ToolParameterDefinitionsValue{
			Description: internal.ToPtr(function.Description),
			Type:        function.Parameters.Type,
		}
		tool.ParameterDefinitions[function.Name] = parameterDefinition
	}

	return tool
}
