package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/invopop/jsonschema"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/packages/respjson"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *openai.ChatCompletionNewParams,
	responseType any,
	response *openai.ChatCompletion,
) (<-chan any, <-chan instructor.StreamData, error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *openai.ChatCompletionNewParams, response *openai.ChatCompletion) (<-chan instructor.StreamData, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	default:
		return i.chatSchemaStream(ctx, *request, response)
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (<-chan instructor.StreamData, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
	}
	request.Tools = createOpenAITools(schema, i.Mode() == instructor.ModeToolCallStrict)
	return i.createStream(ctx, request, response, false)
}

func (i *Instructor) chatSchemaStream(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion) (<-chan instructor.StreamData, error) {
	for idx, msg := range request.Messages {
		if system := msg.OfSystem; system != nil {
			if bs := i.StreamEncoder().Context(); bs != nil {
				system.Content.OfString = openai.String(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", system.Content.OfString.String(), string(bs)))
				request.Messages[idx] = msg
			}
		}
	}
	// Set JSON mode
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		if i.Mode() == instructor.ModeJSONSchema || i.Mode() == instructor.ModeJSONStrict {
			schema := enc.Schema()
			structName := schema.NameFromRef()
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

			request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
					JSONSchema: openai.ResponseFormatJSONSchemaJSONSchemaParam{
						Name:        structName,
						Description: openai.String(schema.Description),
						Schema:      schemaRaw,
						Strict:      openai.Bool(i.Mode() == instructor.ModeJSONStrict),
					},
				},
			}
		} else {
			request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
				OfJSONObject: new(openai.ResponseFormatJSONObjectParam),
			}
		}
	} else {
		request.ResponseFormat = openai.ChatCompletionNewParamsResponseFormatUnion{
			OfText: new(openai.ResponseFormatTextParam),
		}
	}
	return i.createStream(ctx, request, response, false)
}

func (i *Instructor) createStream(ctx context.Context, request openai.ChatCompletionNewParams, response *openai.ChatCompletion, toolRequest bool) (<-chan instructor.StreamData, error) {
	if !toolRequest {
		i.InjectMCP(ctx, &request)
	}
	request.StreamOptions.IncludeUsage = openai.Bool(true)
	extraFields := request.ExtraFields()
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
		if extraFields == nil {
			extraFields = make(map[string]any, 2)
		}
		if thinking.Enabled {
			extraFields["enable_thinking"] = true
			extraFields["thinking"] = "enabled"
		} else {
			extraFields["enable_thinking"] = false
			extraFields["thinking"] = "disabled"
		}
		extraFields["chat_template_kwargs"] = map[string]any{
			"enable_thinking": thinking.Enabled,
			"thinking_budget": thinking.Budget,
		}
	}
	request.SetExtraFields(extraFields)

	if i.Verbose() {
		bs, _ := request.MarshalJSON()
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream := i.Client.Chat.Completions.NewStreaming(ctx, request)

	ch := make(chan instructor.StreamData)

	go func() {
		defer stream.Close()
		defer close(ch)
		bs := new(bytes.Buffer)
		var toolCalls []openai.ChatCompletionMessageToolCall
		if i.Verbose() && !toolRequest {
			defer func() {
			}()
		}
		defer func() {
			if len(toolCalls) == 0 {
				if !toolRequest {
					if txt := bs.String(); txt != "" {
						i.memory.Add(openai.AssistantMessage(txt))
					}
				}
				if i.Verbose() {
					log.Printf("%s Response: %s\n", i.Provider(), bs.String())
				}
				return
			}
			toolCallParams := make([]openai.ChatCompletionMessageToolCallParam, 0, len(toolCalls))
			for _, toolCall := range toolCalls {
				param := openai.ChatCompletionMessageToolCallParam{
					ID: toolCall.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				}
				toolCallParams = append(toolCallParams, param)
			}
			assistantMessage := openai.ChatCompletionMessageParamUnion{
				OfAssistant: &openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCallParams,
				},
			}
			oldMessagesCount := len(request.Messages)
			request.Messages = append(request.Messages, assistantMessage)
			for _, toolCall := range toolCalls {
				if call := i.CallMCP(ctx, &toolCall, &request); call != nil {
					ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: call}
				} else {
					var shouldReturn bool
					for _, tool := range request.Tools {
						if tool.Function.Name == toolCall.Function.Name {
							callReq := new(mcp.CallToolRequest)
							callReq.Params.Name = toolCall.Function.Name
							callReq.Params.Arguments = toolCall.Function.Arguments
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
			}
			if newMessagesCount := len(request.Messages); newMessagesCount > oldMessagesCount && i.memory != nil {
				i.memory.Add(request.Messages[oldMessagesCount:newMessagesCount]...)
			}
			var usage openai.CompletionUsage
			if response != nil {
				usage = response.Usage
			}
			tmpCh, err := i.createStream(ctx, request, response, true)
			if response != nil {
				response.Usage.PromptTokens += usage.PromptTokens
				response.Usage.CompletionTokens += usage.CompletionTokens
				response.Usage.TotalTokens += usage.TotalTokens
			}
			if err != nil {
				ch <- instructor.StreamData{Type: instructor.ErrorStream, Err: err}
				return
			}
			for v := range tmpCh {
				if i.Verbose() && v.Type == instructor.ContentStream {
					bs.WriteString(v.Content)
				}
				ch <- v
			}
		}()
		var acc openai.ChatCompletionAccumulator
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			if tool, ok := acc.JustFinishedToolCall(); ok {
				toolCall := openai.ChatCompletionMessageToolCall{
					ID:       tool.ID,
					Function: tool.ChatCompletionMessageToolCallFunction,
				}
				toolCalls = append(toolCalls, toolCall)
			}
			if chunk.JSON.Usage.Valid() && response != nil {
				response.ID = chunk.ID
				response.Model = chunk.Model
				response.Created = chunk.Created
				response.SystemFingerprint = chunk.SystemFingerprint
				response.Usage = chunk.Usage
			}
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if field, ok := delta.JSON.ExtraFields["reasoning_content"]; ok && field.Raw() != respjson.Omitted && field.Raw() != respjson.Null {
					text := field.Raw()
					ch <- instructor.StreamData{Type: instructor.ThinkingStream, Content: text[1 : len(text)-1]}
				} else if text := delta.Content; text != "" {
					if i.Verbose() {
						bs.WriteString(text)
					}
					ch <- instructor.StreamData{Type: instructor.ContentStream, Content: text}
				}
			}
		}
		if err := stream.Err(); err != nil && i.Verbose() {
			ch <- instructor.StreamData{Type: instructor.ErrorStream, Err: err}
		}
	}()
	return ch, nil
}
