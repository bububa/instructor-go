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
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
	}
	request.Tools = createOpenAITools(schema, strict)
	return i.createStream(ctx, &request, response)
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
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) createStream(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan instructor.StreamData, error) {
	request.Stream = true
	if request.StreamOptions == nil {
		request.StreamOptions = new(openai.StreamOptions)
	}
	request.StreamOptions.IncludeUsage = true
	if extraBody := i.ExtraBody(); extraBody != nil {
		request.ExtraBody = extraBody
	}
	if thinking := i.ThinkingConfig(); thinking != nil {
		request.Thinking = openai.ThinkingTypeEnabled
  } else {
		request.Thinking = openai.ThinkingTypeDisabled
	}

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.CreateChatCompletionStream(ctx, *request)
	if err != nil {
		return nil, err
	}

	ch := make(chan instructor.StreamData)

	go func() {
		defer stream.Close()
		defer close(ch)
		bs := new(bytes.Buffer)
		if i.Verbose() {
			fmt.Fprintf(bs, "%s Response: \n", i.Provider())
			defer func() {
				log.Println(bs.String())
			}()
		}
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
