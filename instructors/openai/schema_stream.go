package openai

import (
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
) (stream <-chan any, thinking <-chan string, err error) {
	stream, thinking, err = chat.SchemaStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, nil, err
	}

	return stream, nil, err
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan string, <-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall:
		return i.chatToolCallStream(ctx, *request, response, false)
	case instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response, true)
	default:
		return i.chatSchemaStream(ctx, *request, response)
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse, strict bool) (<-chan string, <-chan string, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, nil, errors.New("encoder must be JSON Encoder")
	}
	request.Tools = createOpenAITools(schema, strict)
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) chatSchemaStream(ctx context.Context, request openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan string, <-chan string, error) {
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

func (i *Instructor) createStream(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan string, <-chan string, error) {
	request.Stream = true
	if request.StreamOptions == nil {
		request.StreamOptions = new(openai.StreamOptions)
	}
	request.StreamOptions.IncludeUsage = true

	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.CreateChatCompletionStream(ctx, *request)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan string)
	thinkingCh := make(chan string)

	go func() {
		defer stream.Close()
		defer close(thinkingCh)
		defer close(ch)
		if i.Verbose() {
			log.Printf("%s Response: \n", i.Provider())
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
				text := resp.Choices[0].Delta.Content
				ch <- text
			}
		}
	}()
	return ch, thinkingCh, nil
}
