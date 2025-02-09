package cohere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go"
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

func (i *Instructor) Handler(ctx context.Context, request *cohere.ChatRequest, schema *instructor.Schema, response *cohere.NonStreamedChatResponse) (string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCall(ctx, *request, schema, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema:
		return i.chatJSON(ctx, *request, schema, response)
	default:
		return "", fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCall(ctx context.Context, request cohere.ChatRequest, schema *instructor.Schema, response *cohere.NonStreamedChatResponse) (string, error) {
	request.Tools = []*cohere.Tool{createCohereTools(schema)}

	resp, err := i.Client.Chat(ctx, &request)
	if err != nil {
		return "", err
	}
	for _, c := range resp.ToolCalls {
		toolInput, err := json.Marshal(c.Parameters)
		if err != nil {
			i.EmptyResponseWithResponseUsage(response, resp)
			return "", err
		}
		// TODO: handle more than 1 tool use
		if response != nil {
			*response = *resp
		}
		return string(toolInput), nil
	}
	i.EmptyResponseWithResponseUsage(response, resp)
	return "", errors.New("more than 1 tool response at a time is not implemented")
}

func (i *Instructor) chatJSON(ctx context.Context, request cohere.ChatRequest, schema *instructor.Schema, response *cohere.NonStreamedChatResponse) (string, error) {
	i.addOrConcatJSONSystemPrompt(&request, schema)

	resp, err := i.Client.Chat(ctx, &request)
	if err != nil {
		return "", err
	}
	*response = *resp
	return resp.Text, nil
}

func (i *Instructor) addOrConcatJSONSystemPrompt(request *cohere.ChatRequest, schema *instructor.Schema) {
	schemaPrompt := fmt.Sprintf("```json!Please respond with JSON in the following JSON schema - make sure to return an instance of the JSON, not the schema itself: %s ", schema.String)

	if request.Preamble == nil {
		request.Preamble = &schemaPrompt
	} else {
		request.Preamble = internal.ToPtr(*request.Preamble + "\n" + schemaPrompt)
	}
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
		usage.InputTokens += int(*v)
	}
	if v := response.Meta.Tokens.OutputTokens; v != nil {
		usage.OutputTokens += int(*v)
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
