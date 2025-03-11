package cohere

import (
	"context"
	"fmt"
	"reflect"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/internal"
	cohere "github.com/cohere-ai/cohere-go/v2"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan string, error) {
	req := *request
	if (i.Mode() == instructor.ModeJSON || i.Mode() == instructor.ModeJSONSchema) && responseType != nil {
		t := reflect.TypeOf(responseType)

		schema, err := instructor.NewSchema(t)
		if err != nil {
			return nil, err
		}
		i.addOrConcatStreamJSONSystemPrompt(&req, schema)
	}
	stream, err := i.createStream(ctx, &req, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}

func (i *Instructor) addOrConcatStreamJSONSystemPrompt(request *cohere.ChatStreamRequest, schema *instructor.Schema) {
	schemaPrompt := fmt.Sprintf("```json!Please respond with JSON in the following JSON schema - make sure to return an instance of the JSON, not the schema itself: %s ", schema.String)

	if request.Preamble == nil {
		request.Preamble = &schemaPrompt
	} else {
		request.Preamble = internal.ToPtr(*request.Preamble + "\n" + schemaPrompt)
	}
}
