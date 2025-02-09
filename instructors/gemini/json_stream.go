package gemini

import (
	"context"
	"fmt"

	gemini "github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
	"github.com/bububa/instructor-go/internal/chat"
)

func (i *Instructor) JSONStream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (stream <-chan any, err error) {
	stream, err = chat.JSONStreamHandler(i, ctx, request, responseType, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) JSONStreamHandler(ctx context.Context, request *Request, schema *instructor.Schema, response *gemini.GenerateContentResponse) (<-chan string, error) {
	switch i.Mode() {
	case instructor.ModeToolCall, instructor.ModeToolCallStrict:
		return i.chatToolCallStream(ctx, *request, schema, response)
	case instructor.ModeJSON:
		return i.chatJSONStream(ctx, *request, schema, response, false)
	case instructor.ModeJSONStrict, instructor.ModeJSONSchema:
		return i.chatJSONStream(ctx, *request, schema, response, true)
	default:
		return nil, fmt.Errorf("mode '%s' is not supported for %s", i.Mode(), i.Provider())
	}
}

func (i *Instructor) chatToolCallStream(ctx context.Context, request Request, schema *instructor.Schema, response *gemini.GenerateContentResponse) (<-chan string, error) {
	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System
	model.Tools = createTools(schema)

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

func (i *Instructor) chatJSONStream(ctx context.Context, request Request, schema *instructor.Schema, response *gemini.GenerateContentResponse, strict bool) (<-chan string, error) {
	request.Parts = internal.Prepend(request.Parts, createJSONMessageStream(schema))

	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System
	if strict {
		model.ResponseSchema = new(gemini.Schema)
		convertSchema(schema.Schema, model.ResponseSchema)
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

func createJSONMessageStream(schema *instructor.Schema) gemini.Part {
	return gemini.Text(fmt.Sprintf(`
Please respond with a JSON array where the elements following JSON schema:

%s

Make sure to return an array with the elements an instance of the JSON, not the schema itself.
`, schema.String))
}

func (i *Instructor) createStream(ctx context.Context, iter *gemini.GenerateContentResponseIterator, response *gemini.GenerateContentResponse) (<-chan string, error) {
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
