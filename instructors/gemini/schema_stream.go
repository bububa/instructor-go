package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log"

	"google.golang.org/api/iterator"
	gemini "google.golang.org/genai"

	"github.com/bububa/instructor-go"
	jsonenc "github.com/bububa/instructor-go/encoding/json"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) SchemaStream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (stream <-chan any, thinking <-chan string, err error) {
	return chat.SchemaStreamHandler(i, ctx, request, responseType, response)
}

func (i *Instructor) SchemaStreamHandler(ctx context.Context, request *Request, response *gemini.GenerateContentResponse) (<-chan string, <-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, response)
	case instructor.ModeJSON:
		return i.chatJSONStream(ctx, *request, response, false)
	case instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, response, true)
	default:
		return nil, nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse) (<-chan string, <-chan string, error) {
	var schema *instructor.Schema
	if enc, ok := i.StreamEncoder().(*jsonenc.StreamEncoder); ok {
		schema = enc.Schema()
	} else {
		return nil, nil, errors.New("encoder must be JSON Encoder")
	}
	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
		Tools:             createTools(schema),
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
	}

	contents := make([]*gemini.Content, len(request.History)+1)
	contents = append(contents, request.History...)
	contents = append(contents, &gemini.Content{
		Parts: request.Parts,
		Role:  "user",
	})
	iter := i.Models.GenerateContentStream(ctx, request.Model, contents, &cfg)
	return i.createStream(ctx, iter, response)
}

func (i *Instructor) chatJSONStream(ctx context.Context, request Request, response *gemini.GenerateContentResponse, strict bool) (<-chan string, <-chan string, error) {
	if bs := i.StreamEncoder().Context(); bs != nil {
		request.Parts = append(request.Parts, &gemini.Part{Text: string(bs)})
	}
	cfg := gemini.GenerateContentConfig{
		ResponseMIMEType:  "application/json",
		SystemInstruction: request.System,
	}
	if thinkingConfig := i.ThinkingConfig(); thinkingConfig != nil {
		cfg.ThinkingConfig = &gemini.ThinkingConfig{
			IncludeThoughts: true,
		}
	}

	enc, isJSON := i.Encoder().(*jsonenc.Encoder)
	if isJSON {
		cfg.ResponseMIMEType = "application/json"
	} else {
		cfg.ResponseMIMEType = "text/plain"
	}

	if strict {
		cfg.ResponseSchema = new(gemini.Schema)
		schema := enc.Schema()
		convertSchema(schema.Schema, cfg.ResponseSchema)
	}

	if i.Verbose() {
		cfgBytes, _ := json.MarshalIndent(cfg, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Config: %s\n`, i.Provider(), string(bs), string(cfgBytes))
	}
	contents := make([]*gemini.Content, len(request.History)+1)
	contents = append(contents, request.History...)
	contents = append(contents, &gemini.Content{
		Parts: request.Parts,
		Role:  "user",
	})
	iter := i.Models.GenerateContentStream(ctx, request.Model, contents, &cfg)
	return i.createStream(ctx, iter, response)
}

func (i *Instructor) createStream(_ context.Context, iter iter.Seq2[*gemini.GenerateContentResponse, error], response *gemini.GenerateContentResponse) (<-chan string, <-chan string, error) {
	ch := make(chan string)
	thinkingCh := make(chan string)

	go func() {
		defer close(thinkingCh)
		defer close(ch)
		for resp, err := range iter {
			if err == iterator.Done {
				return
			}
			if err != nil {
				return
			}
			response.UsageMetadata = resp.UsageMetadata
			for _, cand := range resp.Candidates {
				if cand.Content == nil {
					continue
				}
				for _, part := range cand.Content.Parts {
					if part.Thought {
						thinkingCh <- part.Text
					} else if text := part.Text; text != "" {
						ch <- text
					}
				}
			}
		}
	}()
	return ch, thinkingCh, nil
}
