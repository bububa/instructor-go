package anthropic

import (
	"context"
	"reflect"

	"github.com/bububa/instructor-go"
	anthropic "github.com/liushuangls/go-anthropic/v2"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *anthropic.MessagesRequest,
	responseType any,
	response *anthropic.MessagesResponse,
) (stream <-chan string, err error) {
	req := *request
	if (i.Mode() == instructor.ModeJSON || i.Mode() == instructor.ModeJSONSchema) && responseType != nil {
		t := reflect.TypeOf(responseType)

		schema, err := instructor.NewSchema(t)
		if err != nil {
			return nil, err
		}
		i.appendJSONSchema(&req, schema)
	}
	stream, err = i.createStream(ctx, &req, response)
	if err != nil {
		return nil, err
	}

	return stream, err
}
