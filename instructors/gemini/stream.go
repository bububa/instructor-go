package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
	gemini "github.com/google/generative-ai-go/genai"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *Request,
	responseType any,
	response *gemini.GenerateContentResponse,
) (stream <-chan string, err error) {
	model := i.GenerativeModel(request.Model)
	model.SystemInstruction = request.System

	if i.Verbose() {
		modelBytes, _ := json.MarshalIndent(model, "", "  ")
		bs, _ := json.MarshalIndent(request, "", "  ")
		log.Printf(`%s Request: %s
      Request Model: %s\n`, i.Provider(), string(bs), string(modelBytes))
	}

	req := *request
	if (i.Mode() == instructor.ModeJSON || i.Mode() == instructor.ModeJSONSchema) && responseType != nil {
		t := reflect.TypeOf(responseType)

		schema, err := instructor.NewSchema(t)
		if err != nil {
			return nil, err
		}

		req.Parts = internal.Prepend(request.Parts, createStreamJSONMessage(schema))
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

func createStreamJSONMessage(schema *instructor.Schema) gemini.Part {
	return gemini.Text(fmt.Sprintf("\nPlease respond with a JSON array where the elements following JSON schema:\n```json\n%s\n```\nMake sure to return an array with the elements an instance of the JSON, not the schema itself.\n", schema.String))
}
