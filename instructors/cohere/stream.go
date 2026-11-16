package cohere

import (
	"context"
	"fmt"

	cohere "github.com/cohere-ai/cohere-go/v2"

	"github.com/bububa/instructor-go"
	"github.com/bububa/instructor-go/encoding"
)

func (i *Instructor) Stream(
	ctx context.Context,
	request *cohere.ChatStreamRequest,
	responseType any,
	response *cohere.NonStreamedChatResponse,
) (<-chan instructor.StreamData, error) {
	req := *request
	if responseType != nil {
		if i.Encoder() == nil {
			if enc, err := encoding.PredefinedEncoder(i.Mode(), responseType, i.SchemaNamer()); err != nil {
				return nil, err
			} else {
				i.SetEncoder(enc)
			}
		}
		if bs := i.Encoder().Context(); bs != nil {
			if system := req.Preamble; system == nil {
				req.Preamble = new(string(bs))
			} else {
				req.Preamble = new(fmt.Sprintf("%s\n\n#OUTPUT SCHEMA\n%s", *system, string(bs)))
			}
		}
	}
	return i.createStream(ctx, &req, response)
}
