package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"

	// "github.com/bububa/ljson"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"

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
	request *openai.ChatCompletionNewParams,
	responseType any,
	response *openai.ChatCompletion,
) error {
	req := *request
	extraFields := req.ExtraFields()
	if extraBody := i.ExtraBody(); extraBody != nil {
		if extraFields == nil {
			extraFields = map[string]any{
				"extra_body": extraBody,
			}
		} else if _, ok := extraFields["extra_body"]; !ok {
			extraFields["extra_body"] = extraBody
		}
	}
	if thinking := i.ThinkingConfig(); thinking != nil {
		if kv := thinking.Marshaler; kv != nil {
			if extraFields != nil {
				maps.Copy(extraFields, kv())
			} else {
				extraFields = kv()
			}
		} else {
			if extraFields == nil {
				extraFields = make(map[string]any, 2)
			}
			if thinking.Enabled {
				extraFields["enable_thinking"] = true
				extraFields["thinking"] = map[string]string{
					"type": "enabled",
				}
			} else {
				extraFields["enable_thinking"] = false
				extraFields["thinking"] = map[string]string{
					"type": "disabled",
				}
			}
			extraFields["chat_template_kwargs"] = map[string]any{
				"enable_thinking": thinking.Enabled,
				"thinking_budget": thinking.Budget,
			}
		}
	}
	req.SetExtraFields(extraFields)
	return chat.Handler(i, ctx, &req, responseType, response)
}

func (i *Instructor) Handler(ctx context.Context, request *openai.ChatCompletionNewParams, response *openai.ChatCompletion) (string, error) {
	req := *request
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCall(ctx, req, response)
	case instructor.ModeJSON, instructor.ModeJSONSchema, instructor.ModeJSONStrict:
		return i.chatJSON(ctx, req, response)
	default:
		return i.chatCompletion(ctx, req, response)
	}
}

func (i *Instructor) chatToolCall(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	request.Tools = createOpenAITools(schema, i.Mode() == instructor.ModeToolCallStrict)
	if i.Verbose() {
		bs, _ := request.MarshalJSON()
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}

	memory := i.Memory()
	if memory != nil && len(request.Messages) > 0 {
		var msg instructor.Message
		if err := ConvertMessageTo(&request.Messages[len(request.Messages)-1], &msg); err == nil {
			memory.Add(msg)
		}
	}
	resp, err := i.Client.Chat.Completions.New(ctx, request)
	if err != nil {
		return "", err
	}

	var toolCalls []openai.ChatCompletionMessageToolCall
	for _, choice := range resp.Choices {
		toolCalls = choice.Message.ToolCalls

		if len(toolCalls) >= 1 {

			if memory != nil {
				msg := instructor.Message{
					Role: instructor.AssistantRole,
					ToolUses: []instructor.ToolUse{
						{
							ID:        toolCalls[0].ID,
							Name:      toolCalls[0].Function.Name,
							Arguments: toolCalls[0].Function.Arguments,
						},
					},
				}
				memory.Add(msg)
			}
			break
		}
	}

	numTools := len(toolCalls)

	if numTools < 1 {
		i.EmptyResponseWithResponseUsage(response, resp)
		return "", errors.New("received no tool calls from model, expected at least 1")
	}

	if numTools == 1 {
		if response != nil {
			*response = *resp
		}
		return toolCalls[0].Function.Arguments, nil
	}

	// numTools >= 1

	jsonArray := make([]map[string]any, len(toolCalls))

	for idx, toolCall := range toolCalls {
		var jsonObj map[string]any
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &jsonObj); err != nil {
			i.EmptyResponseWithResponseUsage(response, resp)
			return "", err
		}
		jsonArray[idx] = jsonObj
	}

	resultJSON, err := json.Marshal(jsonArray)
	if err != nil {
		i.EmptyResponseWithResponseUsage(response, resp)
		return "", err
	}

	if response != nil {
		*response = *resp
	}
	return string(resultJSON), nil
}

func (i *Instructor) chatJSON(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (string, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return "", errors.New("encoder must be JSON Encoder")
	}
	structName := schema.NameFromRef()

	var (
		hasSystem bool
		lastIdx   = -1
	)
	for idx, msg := range request.Messages {
		if system := msg.OfSystem; system != nil {
			bs := i.Encoder().Context()
			if bs != nil {
				system.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", system.Content.OfString.Value, string(bs)))
				request.Messages[idx] = msg
				hasSystem = true
			}
		}
		lastIdx = idx
	}

	if i.Mode() == instructor.ModeJSONSchema || i.Mode() == instructor.ModeJSONStrict {
		request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:        structName,
					Description: openai.String(schema.Description),
					Schema:      schema,
					Strict:      openai.Bool(i.Mode() == instructor.ModeJSONStrict),
				},
			},
		}
	} else {
		if !hasSystem && lastIdx >= 0 {
			bs := i.Encoder().Context()
			if msg := request.Messages[lastIdx].OfUser; msg != nil {
				request.Messages[lastIdx].OfUser.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content.OfString.Value, string(bs)))
			}
		}
		request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONObject: new(openai.ResponseFormatJSONObjectParam),
		}
	}
	memory := i.Memory()
	if memory != nil && lastIdx >= 0 {
		var msg instructor.Message
		if err := ConvertMessageTo(&request.Messages[lastIdx], &msg); err == nil {
			memory.Add(msg)
		}
	}

	text, err := i.chatCompletionWrapper(ctx, request, response)
	if err != nil {
		return "", err
	}

	// if i.Mode() == instructor.ModeJSONStrict || i.Mode() == instructor.ModeJSONSchema {
	// 	resMap := make(map[string]any)
	// 	_ = ljson.Unmarshal([]byte(text), &resMap)
	//
	// 	cleanedText, _ := json.Marshal(resMap[structName])
	// 	text = string(cleanedText)
	// }
	return text, nil
}

func (i *Instructor) chatCompletion(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (string, error) {
	lastIdx := -1
	for idx, msg := range request.Messages {
		if system := msg.OfSystem; system != nil {
			bs := i.Encoder().Context()
			if bs != nil {
				system.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", system.Content.OfString.Value, string(bs)))
				request.Messages[idx] = msg
			}
		}
		lastIdx = idx
	}
	memory := i.Memory()
	if memory != nil && lastIdx >= 0 {
		var msg instructor.Message
		if err := ConvertMessageTo(&request.Messages[lastIdx], &msg); err == nil {
			memory.Add(msg)
		}
	}
	// request.Messages = internal.Prepend(request.Messages, *createJSONMessage(schema))
	return i.chatCompletionWrapper(ctx, request, response)
}

func (i *Instructor) chatCompletionWrapper(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (string, error) {
	i.InjectMCP(ctx, &request)
	if i.Verbose() {
		bs, _ := request.MarshalJSON()
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	memory := i.Memory()
	resp, err := i.Client.Chat.Completions.New(ctx, request)
	if err != nil {
		return "", err
	}
	if i.Verbose() {
		log.Printf("%s Response: %s\n", i.Provider(), resp.RawJSON())
	}
	if response != nil {
		*response = *resp
	}
	toolCalls := resp.Choices[0].Message.ToolCalls
	if len(toolCalls) > 0 {
		oldMessagesCount := len(request.Messages)
		request.Messages = append(request.Messages, resp.Choices[0].Message.ToParam())
		for _, toolCall := range toolCalls {
			if call := i.CallMCP(ctx, &toolCall, &request); call != nil && i.Verbose() {
				bs, _ := json.MarshalIndent(call, "", "  ")
				log.Printf("%s ToolCall Result: %s\n", i.Provider(), string(bs))
			}
		}
		if newMessagesCount := len(request.Messages); newMessagesCount > oldMessagesCount && memory != nil {
			for _, v := range request.Messages[oldMessagesCount:newMessagesCount] {
				var msg instructor.Message
				if err := ConvertMessageTo(&v, &msg); err == nil {
					memory.Add(msg)
				}
			}
		}
		var usage openai.CompletionUsage
		if response != nil {
			*response = *resp
			usage = response.Usage
		}
		text, err := i.chatCompletionWrapper(ctx, request, response)
		if response != nil {
			response.Usage.PromptTokens += usage.PromptTokens
			response.Usage.CompletionTokens += usage.CompletionTokens
			response.Usage.TotalTokens += usage.TotalTokens
		}
		return text, err
	}
	text := resp.Choices[0].Message.Content
	if memory != nil {
		memory.Add(instructor.Message{
			Role: instructor.AssistantRole,
			Text: text,
		})
	}
	return text, nil
}

func (i *Instructor) EmptyResponseWithUsageSum(ret *openai.ChatCompletion, usage *instructor.UsageSum) {
	if ret == nil || usage == nil {
		return
	}
	*ret = openai.ChatCompletion{
		Usage: openai.CompletionUsage{
			PromptTokens:     usage.InputTokens,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      usage.TotalTokens,
		},
	}
}

func (i *Instructor) EmptyResponseWithResponseUsage(ret *openai.ChatCompletion, response *openai.ChatCompletion) {
	if ret == nil {
		return
	}
	if response == nil {
		*ret = openai.ChatCompletion{}
		return
	}

	*ret = openai.ChatCompletion{
		Usage: response.Usage,
	}
}

func (i *Instructor) SetUsageSumToResponse(response *openai.ChatCompletion, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	response.Usage.PromptTokens = usage.InputTokens
	response.Usage.CompletionTokens = usage.OutputTokens
	response.Usage.TotalTokens = usage.TotalTokens
}

func (i *Instructor) CountUsageFromResponse(response *openai.ChatCompletion, usage *instructor.UsageSum) {
	if response == nil || usage == nil {
		return
	}
	usage.InputTokens += response.Usage.PromptTokens
	usage.OutputTokens += response.Usage.CompletionTokens
	usage.TotalTokens += response.Usage.TotalTokens
}

func createOpenAITools(schema *instructor.Schema, strict bool) []openai.ChatCompletionToolParam {
	tools := make([]openai.ChatCompletionToolParam, 0, len(schema.Functions))
	for _, function := range schema.Functions {
		f := openai.FunctionDefinitionParam{
			Name:        function.Name,
			Description: openai.String(function.Description),
			Parameters: openai.FunctionParameters{
				"type":       function.Parameters.Type,
				"required":   function.Parameters.Required,
				"properties": function.Parameters.Properties,
			},
			Strict: openai.Bool(strict),
		}
		t := openai.ChatCompletionToolParam{
			Function: f,
		}
		tools = append(tools, t)
	}
	return tools
}
