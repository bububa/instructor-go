package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	gemini "github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (stream <-chan any, err error) {
	stream, err = chat.SchemaStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *Request, response *gemini.GenerateContentResponse) (<-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	case instructor.ModeJSON:
		return i.chatJSONStream(ctx, *request, response, false)
	case instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, response, true)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse) (<-chan string, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, errors.New("encoder must be JSON Encoder")
	}
	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System
	model.Tools = createTools(schema)

	if i.Verbose() {
		modelBytes, _ := json.MarshalIndent(model, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Model: %s\n`, i.Provider(), string(bs), string(modelBytes))
	}

	var iter *gemini.GenerateContentResponseIterator

	if len(request.History) > 0 {
		cs := model.StartChat()
		cs.History = request.History
		iter = cs.SendMessageStream(ctx, request.Parts...)
	} else {
		iter = model.GenerateContentStream(ctx, request.Parts...)
	}
	return i.createStream(ctx, iter, response)
}

func (i *Instructor) chatJSONStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse, strict bool) (<-chan string, error) {
	if bs := i.StreamEncoder().Context(); bs != nil {
		request.Parts = append(request.Parts, gemini.Text(string(bs)))
	}

	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System
	enc, isJSON := i.StreamEncoder().(*jsonenc.StreamEncoder)
	if isJSON {
		model.ResponseMIMEType = "application/json"
	} else {
		model.ResponseMIMEType = "text/plain"
	}
	if strict {
		model.ResponseSchema = new(gemini.Schema)
		schema := enc.Schema()
		convertSchema(schema.Schema, model.ResponseSchema)
	}

	if i.Verbose() {
		modelBytes, _ := json.MarshalIndent(model, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Model: %s\n`, i.Provider(), string(bs), string(modelBytes))
	}

	var iter *gemini.GenerateContentResponseIterator

	if len(request.History) > 0 {
		cs := model.StartChat()
		cs.History = request.History
		iter = cs.SendMessageStream(ctx, request.Parts...)
	} else {
		iter = model.GenerateContentStream(ctx, request.Parts...)
	}
	return i.createStream(ctx, iter, response)
}

func (i *Instructor) createStream(_ context.Context, iter *gemini.GenerateContentResponseIterator, response *gemini.GenerateContentResponse) (<-chan string, error) {
	ch := make(chan string)

	go func() {
		defer close(ch)
		defer func() {
			*response = *iter.MergedResponse()
		}()
		for {
			resp, err := iter.Next()
			if err == iterator.Done {
				return
			}
			if err != nil {
				return
			}
			for _, cand := range resp.Candidates {
				if cand.Content == nil {
					continue
				}
				for _, part := range cand.Content.Parts {
					if text, ok := part.(gemini.Text); ok {
						ch <- string(text)
					}
				}
			}
		}
	}()
	return ch, nil
}
