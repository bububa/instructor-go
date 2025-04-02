package cohere

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan any, <-chan string, error) {
	stream, thinking, err := chat.SchemaStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, nil, err
	}

	return stream, thinking, err
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *cohere.ChatStreamRequest, response *cohere.NonStreamedChatResponse) (<-chan string, <-chan string, error) {
	if bs := i.StreamEncoder().Context(); bs != nil {
		if system := request.Preamble; system == nil {
			request.Preamble = internal.ToPtr(string(bs))
		} else {
			request.Preamble = internal.ToPtr(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", *system, string(bs)))
		}
	}
	return i.createStream(ctx, request, response)
}

func (i *Instructor) createStream(ctx context.Context, request *cohere.ChatStreamRequest, response *cohere.NonStreamedChatResponse) (<-chan string, <-chan string, error) {
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.ChatStream(ctx, request)
	if err != nil {
		return nil, nil, err
	}

	ch := make(chan string)
	thinkingCh := make(chan string)

	go func() {
		defer stream.Close()
		defer close(thinkingCh)
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
	return ch, thinkingCh, nil
}
