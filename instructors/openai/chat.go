package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/bububa/ljson"
	"github.com/invopop/jsonschema"
	openai "github.com/sashabaranov/go-openai"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

type ResponseFormatSchemaWrapper struct {
	Type                 string                  `json:"type"`
	Required             []string                `json:"required"`
	AdditionalProperties bool                    `json:"additionalProperties"`
	Properties           *jsonschema.Definitions `json:"properties"`
	Definitions          *jsonschema.Definitions `json:"$defs"`
}

func (i *Instructor) Chat(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
) error {
	return chat.Handler(i, ctx, request, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (string, error) {
	req := *request
	if req.ExtraBody == nil {
		if extraBody := i.ExtraBody(); extraBody != nil {
			req.ExtraBody = extraBody
		}
	}
	if thinking := i.ThinkingConfig(); thinking != nil {
		if thinking.Enabled {
			req.Thinking = openai.ThinkingTypeEnabled
		} else {
			req.Thinking = openai.ThinkingTypeDisabled
		}
		req.ChatTemplateKwargs = map[string]any{
			"enable_thinking": thinking.Enabled,
			"thinking_budget": thinking.Budget,
		}
	}
	switch i.Mode() {
	case instructor.ModeToolCall:
		return i.chatToolCall(ctx, req, response, false)
	case instructor.ModeToolCallStrict:
		return i.chatToolCall(ctx, req, response, true)
	case instructor.ModeJSON:
		return i.chatJSON(ctx, req, response, false)
	case instructor.ModeJSONStrict:
		return i.chatJSON(ctx, req, response, true)
	default:
		return i.chatCompletion(ctx, req, response)
	}
}

func (i *Instructor) chatToolCall(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse, strict bool) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	request.Stream = false
	request.Tools = createOpenAITools(schema, strict)
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	var toolCalls []openai.ToolCall
	for _, choice := range resp.Choices {
		toolCalls = choice.Message.ToolCalls

		if len(toolCalls) >= 1 {
			break
		}
	}

	numTools := len(toolCalls)

	if numTools < 1 {
		i.EmptyResponseWithResponseUsage(response, &resp)
		return "", errors.New("received no tool calls from model, expected at least 1")
	}

	if numTools == 1 {
		if response != nil {
			*response = resp
		}
		return toolCalls[0].Function.Arguments, nil
	}

	// numTools >= 1

	jsonArray := make([]map[string]any, len(toolCalls))

	for idx, toolCall := range toolCalls {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &jsonObj); err != nil {
			i.EmptyResponseWithResponseUsage(response, &resp)
			return "", err
		}
		jsonArray[idx] = jsonObj
	}

	resultJSON, err := json.Marshal(jsonArray)
	if err != nil {
		i.EmptyResponseWithResponseUsage(response, &resp)
		return "", err
	}

	if response != nil {
		*response = resp
	}
	return string(resultJSON), nil
}

func (i *Instructor) chatJSON(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse, strict bool) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	structName := schema.NameFromRef()

	request.Stream = false
	for idx, msg := range request.Messages {
		if msg.Role == "system" {
			bs := i.Encoder().Context()
			if bs != nil {
				request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, string(bs))
			}
		}
	}

	if strict {
		schemaWrapper := ResponseFormatSchemaWrapper{
			Type:        "object",
			Required:    []string{structName},
			Definitions: &schema.Definitions,
			Properties: &jsonschema.Definitions{
				structName: schema.Definitions[structName],
			},
			AdditionalProperties: false,
		}

		schemaJSON, _ := json.Marshal(schemaWrapper)
		schemaRaw := json.RawMessage(schemaJSON)

		request.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:        structName,
				Description: schema.Description,
				Schema:      schemaRaw,
				Strict:      true,
			},
		}
	} else {
		request.ResponseFormat = &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		}
	}
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	text := resp.Choices[0].Message.Content

	if strict {
		resMap := make(map[string]any)
		_ = ljson.Unmarshal([]byte(text), &resMap)

		cleanedText, _ := json.Marshal(resMap[structName])
		text = string(cleanedText)
	}
	if response != nil {
		*response = resp
	}
	return text, nil
}

func (i *Instructor) chatCompletion(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (string, error) {
	request.Stream = false
	for idx, msg := range request.Messages {
		if msg.Role == "system" {
			bs := i.Encoder().Context()
			if bs != nil {
				request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, string(bs))
			}
		}
	}
	// request.Messages = internal.Prepend(request.Messages, *createJSONMessage(schema))

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	resp, err := i.CreateChatCompletion(ctx, request)
	if err != nil {
		return "", err
	}

	text := resp.Choices[0].Message.Content
	if response != nil {
		*response = resp
	}
	return text, nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *openai.ChatCompletionResponse, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = openai.ChatCompletionResponse{
		Usage: openai.Usage{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.TotalTokens,
		},
	}
}

func (i *Instructor) EmptyResponseWithResponseUsage(ret *openai.ChatCompletionResponse, response *openai.ChatCompletionResponse) {
	if ret == nil {
		return
	}
	if response == nil {
		*ret = openai.ChatCompletionResponse{}
		return
	}

	*ret = openai.ChatCompletionResponse{
		Usage: response.Usage,
	}
}

func (i *Instructor) SetUsageSumToResponse(response *openai.ChatCompletionResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	response.Usage.PromptTokens = usage.InputTokens
	response.Usage.CompletionTokens = usage.OutputTokens
	response.Usage.TotalTokens = usage.TotalTokens
}

func (i *Instructor) CountUsageFromResponse(response *openai.ChatCompletionResponse, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	usage.InputTokens += response.Usage.PromptTokens
	usage.OutputTokens += response.Usage.CompletionTokens
	usage.TotalTokens += response.Usage.TotalTokens
}

func createOpenAITools(schema *instructor.Schema, strict bool) []openai.Tool {
	tools := make([]openai.Tool, 0, len(schema.Functions))
	for _, function := range schema.Functions {
		f := openai.FunctionDefinition{
			Name:        function.Name,
			Description: function.Description,
			Parameters:  function.Parameters,
			Strict:      strict,
		}
		t := openai.Tool{
			Type:     "function",
			Function: &f,
		}
		tools = append(tools, t)
	}
	return tools
}
