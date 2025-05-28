package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	openai "github.com/sashabaranov/go-openai"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
) (<-chan any, <-chan instructor.StreamData, error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan instructor.StreamData, error) {
	switch i.Mode() {
	case instructor.ModeToolCall:
		return i.chatToolCallStream(ctx, *request, response, false)
	case instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response, true)
	default:
		return i.chatSchemaStream(ctx, *request, response)
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse, strict bool) (<-chan instructor.StreamData, error) {
	var schema *instructor.Schema
	if enc, ok := i.Encoder().(*jsonenc.Encoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
	}
	request.Tools = createOpenAITools(schema, strict)
	return i.createStream(ctx, request, response, false)
}

func (i *Instructor) chatSchemaStream(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan instructor.StreamData, error) {
	for idx, msg := range request.Messages {
		if msg.Role == "system" {
			if bs := i.StreamEncoder().Context(); bs != nil {
				request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, string(bs))
			}
		}
	}
	// Set JSON mode
	if _, ok := i.Encoder().(*jsonenc.Encoder); ok {
		request.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	} else {
		request.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeText}
	}
	return i.createStream(ctx, request, response, false)
}

func (i *Instructor) createStream(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse, toolRequest bool) (<-chan instructor.StreamData, error) {
	i.InjectMCP(ctx, &request)
	request.Stream = true
	if request.StreamOptions == nil {
		request.StreamOptions = new(openai.StreamOptions)
	}
	request.StreamOptions.IncludeUsage = true
	if extraBody := i.ExtraBody(); extraBody != nil {
		request.ExtraBody = extraBody
	}
	if thinking := i.ThinkingConfig(); thinking != nil {
		if thinking.Enabled {
			request.Thinking = &openai.Thinking{
				Type: openai.ThinkingTypeEnabled,
			}
		} else {
			request.Thinking = &openai.Thinking{
				Type: openai.ThinkingTypeDisabled,
			}
		}
		request.ChatTemplateKwargs = map[string]any{
			"enable_thinking": thinking.Enabled,
			"thinking_budget": thinking.Budget,
		}
	}

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.CreateChatCompletionStream(ctx, request)
	if err != nil {
		return nil, err
	}

	ch := make(chan instructor.StreamData)

	go func() {
		defer stream.Close()
		defer close(ch)
		bs := new(bytes.Buffer)
		toolCallMap := make(map[int]openai.ToolCall)
		if i.Verbose() && !toolRequest {
			defer func() {
				fmt.Fprintf(bs, "%s Response: \n", i.Provider())
				log.Println(bs.String())
			}()
		}
		defer func() {
			if len(toolCallMap) > 0 {
				toolCalls := make([]openai.ToolCall, 0, len(toolCallMap))
				for _, toolCall := range toolCallMap {
					toolCalls = append(toolCalls, toolCall)
				}
				request.Messages = append(request.Messages, openai.ChatCompletionMessage{
					Role:      openai.ChatMessageRoleAssistant, // 模型角色
					ToolCalls: toolCalls,
				})
				for _, toolCall := range toolCalls {
					if call := i.CallMCP(ctx, &toolCall, &request); call != nil {
						ch <- instructor.StreamData{Type: instructor.ToolCallStream, ToolCall: call}
					}
				}
				var usage openai.Usage
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
					return
				}
				for v := range tmpCh {
					if i.Verbose() && v.Type == instructor.ContentStream {
						bs.WriteString(v.Content)
					}
					ch <- v
				}
			}
		}()
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				return
			}
			if resp.Usage != nil && response != nil {
				response.ID = resp.ID
				response.Model = resp.Model
				response.Created = resp.Created
				response.SystemFingerprint = resp.SystemFingerprint
				response.PromptFilterResults = resp.PromptFilterResults
				response.Usage = *resp.Usage
			}
			if len(resp.Choices) > 0 {
				delta := resp.Choices[0].Delta
				if tools := delta.ToolCalls; len(tools) > 0 {
					for _, v := range tools {
						if v.Index == nil {
							continue
						}
						toolCall, ok := toolCallMap[*v.Index]
						if !ok {
							toolCall = openai.ToolCall{
								ID:   v.ID,
								Type: openai.ToolTypeFunction,
								Function: openai.FunctionCall{
									Name:      v.Function.Name,
									Arguments: v.Function.Arguments,
								},
							}
						} else {
							toolCall.Function.Arguments += v.Function.Arguments
						}
						if v.Function.Name != "" {
							toolCall.Function.Name = v.Function.Name
						}
						toolCallMap[*v.Index] = toolCall
					}
					// } else if len(toolCallMap) > 0 && delta.Content == "" && delta.Role == "" && delta.FunctionCall == nil {
					// 	break
				}
				if text := resp.Choices[0].Delta.ReasoningContent; text != "" {
					ch <- instructor.StreamData{Type: instructor.ThinkingStream, Content: text}
				} else if text := resp.Choices[0].Delta.Content; text != "" {
					if i.Verbose() {
						bs.WriteString(text)
					}
					ch <- instructor.StreamData{Type: instructor.ContentStream, Content: text}
				}
			}
		}
	}()
	return ch, nil
}
