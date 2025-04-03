package cohere

import (
	"bytes"
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

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan any, <-chan instructor.StreamData, error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *cohere.ChatStreamRequest, response *cohere.NonStreamedChatResponse) (<-chan instructor.StreamData, error) {
	if bs := i.StreamEncoder().Context(); bs != nil {
		if system := request.Preamble; system == nil {
			request.Preamble = internal.ToPtr(string(bs))
		} else {
			request.Preamble = internal.ToPtr(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", *system, string(bs)))
		}
	}
	return i.createStream(ctx, request, response)
}

func (i *Instructor) createStream(ctx context.Context, request *cohere.ChatStreamRequest, response *cohere.NonStreamedChatResponse) (<-chan instructor.StreamData, error) {
	if i.Verbose() {
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf("%s Request: %s\n", i.Provider(), string(bs))
	}
	stream, err := i.ChatStream(ctx, request)
	if err != nil {
		return nil, err
	}

	ch := make(chan instructor.StreamData)

	go func() {
		defer stream.Close()
		defer close(ch)
		sb := new(bytes.Buffer)
		if i.Verbose() {
			fmt.Fprintf(sb, "%s Response: \n", i.Provider())
			defer log.Println(sb.String())
		}
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
				if i.Verbose() {
					sb.WriteString(*message.ToolCallsGeneration.Text)
				}
				ch <- instructor.StreamData{Type: instructor.ContentStream, Content: *message.ToolCallsGeneration.Text}
			case "text-generation":
				if i.Verbose() {
					sb.WriteString(message.TextGeneration.Text)
				}
				ch <- instructor.StreamData{Type: instructor.ContentStream, Content: message.TextGeneration.Text}
			}
		}
	}()
	return ch, nil
}
