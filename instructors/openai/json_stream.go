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
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) JSONStream(
	ctx context.Context,
	request *openai.ChatCompletionRequest,
	responseType any,
	response *openai.ChatCompletionResponse,
) (stream <-chan any, err error) {
	stream, err = chat.JSONStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) JSONStreamHandler(ctx context.Context, request *openai.ChatCompletionRequest, schema *instructor.Schema, response *openai.ChatCompletionResponse) (<-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall:
		return i.chatToolCallStream(ctx, *request, schema, response, false)
	case instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, schema, response, true)
	case instructor.ModeJSON:
		return i.chatJSONStream(ctx, *request, schema, response)
	case instructor.ModeJSONSchema:
		return i.chatJSONSchemaStream(ctx, *request, schema, response)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request openai.ChatCompletionRequest, schema *instructor.Schema, response *openai.ChatCompletionResponse, strict bool) (<-chan string, error) {
	request.Tools = createOpenAITools(schema, strict)
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) chatJSONStream(ctx context.Context, request openai.ChatCompletionRequest, schema *instructor.Schema, response *openai.ChatCompletionResponse) (<-chan string, error) {
	// request.Messages = internal.Prepend(request.Messages, *createJSONMessageStream(schema))
	for idx, msg := range request.Messages {
		if msg.Role == "system" {
			request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, appendJSONMessageStream(schema))
		}
	}
	// Set JSON mode
	request.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) chatJSONSchemaStream(ctx context.Context, request openai.ChatCompletionRequest, schema *instructor.Schema, response *openai.ChatCompletionResponse) (<-chan string, error) {
	// request.Messages = internal.Prepend(request.Messages, *createJSONMessageStream(schema))
	for idx, msg := range request.Messages {
		if msg.Role == "system" {
			request.Messages[idx].Content = fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", msg.Content, appendJSONMessageStream(schema))
		}
	}
	return i.createStream(ctx, &request, response)
}

func appendJSONMessageStream(schema *instructor.Schema) string {
	return fmt.Sprintf("\nPlease respond with a JSON array where the elements following JSON schema:\n```json\n%s\n```\nMake sure to return an array with the elements an instance of the JSON, not the schema itself.\n", schema.String)
}

func (i *Instructor) createStream(ctx context.Context, request *openai.ChatCompletionRequest, response *openai.ChatCompletionResponse) (<-chan string, error) {
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
		return nil, err
	}

	ch := make(chan string)

	go func() {
		defer stream.Close()
		defer close(ch)
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
	return ch, nil
}
