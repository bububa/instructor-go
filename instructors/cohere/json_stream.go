package cohere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) JSONStream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan any, error) {
	stream, err := chat.JSONStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) JSONStreamHandler(ctx context.Context, request *cohere.ChatStreamRequest, schema *instructor.Schema, response *cohere.NonStreamedChatResponse) (<-chan string, error) {
	switch i.Mode() {
	case instructor.ModeJSON, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, schema, response)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatJSONStream(ctx context.Context, request cohere.ChatStreamRequest, schema *instructor.Schema, response *cohere.NonStreamedChatResponse) (<-chan string, error) {
	i.addOrConcatJSONSystemPromptStream(&request, schema)
	return i.createStream(ctx, &request, response)
}

func (i *Instructor) addOrConcatJSONSystemPromptStream(request *cohere.ChatStreamRequest, schema *instructor.Schema) {
	schemaPrompt := fmt.Sprintf("```json!Please respond a JSON array where the elements the following JSON schema - make sure to return an array with the elements an instance of the JSON, not the schema itself: %s ", schema.String)

	if request.Preamble == nil {
		request.Preamble = &schemaPrompt
	} else {
		request.Preamble = internal.ToPtr(*request.Preamble + "\n" + schemaPrompt)
	}
}

func (i *Instructor) createStream(ctx context.Context, request *cohere.ChatStreamRequest, response *cohere.NonStreamedChatResponse) (<-chan string, error) {
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.ChatStream(ctx, request)
	if err != nil {
		return nil, err
	}

	ch := make(chan string)

	go func() {
		defer stream.Close()
		defer close(ch)
		for {
			message, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				return
			}
			switch message.EventType {
			case "stream-start":
				continue
			case "stream-end":
				if response != nil {
					*response = *message.StreamEnd.Response
				}
				return
			case "tool-calls-generation":
				ch <- *message.ToolCallsGeneration.Text
			case "text-generation":
				ch <- message.TextGeneration.Text
			}
		}
	}()
	return ch, nil
}
